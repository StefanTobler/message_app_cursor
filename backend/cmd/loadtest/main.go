package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"sync"
	"time"
)

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

type Message struct {
	ConversationID int64  `json:"conversation_id"`
	Content        string `json:"content"`
}

const (
	NUM_USERS          = 10000
	MESSAGES_PER_SEC   = 1
	SIMULATION_TIME    = 60 // seconds
	BASE_URL          = "http://localhost:8080"
	CONVERSATIONS     = 100 // number of conversations to distribute users across
	BATCH_SIZE        = 100 // number of users to create in parallel
)

func registerUser(id int) (*User, error) {
	payload := map[string]string{
		"username": fmt.Sprintf("loadtest_user_%d", id),
		"password": "testpass123",
		"avatar":   fmt.Sprintf("https://avatar.com/%d", id),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(BASE_URL+"/api/auth/register", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("registration failed with status: %d", resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
		User  User   `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	result.User.Token = result.Token
	return &result.User, nil
}

type OperationType int

const (
	WriteOperation OperationType = iota
	ReadOperation
)

type Stats struct {
	sync.Mutex
	totalRequests     int64
	successRequests   int64
	failedRequests    int64
	totalLatency      time.Duration
	maxLatency        time.Duration
	minLatency        time.Duration
	requestsPerSecond float64
	writeLatencies    []time.Duration // Store write latencies for p99 calculation
	readLatencies     []time.Duration // Store read latencies for p99 calculation
}

func (s *Stats) recordSuccess(latency time.Duration, opType OperationType) {
	s.Lock()
	defer s.Unlock()
	s.totalRequests++
	s.successRequests++
	s.totalLatency += latency
	if latency > s.maxLatency {
		s.maxLatency = latency
	}
	if s.minLatency == 0 || latency < s.minLatency {
		s.minLatency = latency
	}

	switch opType {
	case WriteOperation:
		s.writeLatencies = append(s.writeLatencies, latency)
	case ReadOperation:
		s.readLatencies = append(s.readLatencies, latency)
	}
}

func (s *Stats) recordError() {
	s.Lock()
	defer s.Unlock()
	s.totalRequests++
	s.failedRequests++
}

func (s *Stats) calculateStats(duration time.Duration) {
	s.Lock()
	defer s.Unlock()
	s.requestsPerSecond = float64(s.totalRequests) / duration.Seconds()
}

func createConversation(id int, adminUser *User) error {
	payload := map[string]interface{}{
		"name": fmt.Sprintf("LoadTest Conversation %d", id),
		"type": "group",
		"participants": []int64{adminUser.ID},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", BASE_URL+"/api/conversations/create", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminUser.Token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("conversation creation failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (s *Stats) getP99Latency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Sort latencies
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate p99 index
	p99Index := int(float64(len(sorted)) * 0.99)
	if p99Index >= len(sorted) {
		p99Index = len(sorted) - 1
	}

	return sorted[p99Index]
}

func (s *Stats) getP99WriteLatency() time.Duration {
	s.Lock()
	defer s.Unlock()
	return s.getP99Latency(s.writeLatencies)
}

func (s *Stats) getP99ReadLatency() time.Duration {
	s.Lock()
	defer s.Unlock()
	return s.getP99Latency(s.readLatencies)
}

func simulateUser(user *User, wg *sync.WaitGroup, stats *Stats) {
	defer wg.Done()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	ticker := time.NewTicker(time.Second / MESSAGES_PER_SEC)
	defer ticker.Stop()

	endTime := time.Now().Add(SIMULATION_TIME * time.Second)

	for time.Now().Before(endTime) {
		<-ticker.C

		// Randomly choose between read and write operations
		isWrite := rand.Float32() < 0.5 // 50% chance of write vs read

		if isWrite {
			// Create and send message (write operation)
			msg := Message{
				ConversationID: int64(rand.Intn(CONVERSATIONS) + 1),
				Content:        fmt.Sprintf("Test message from user %d at %s", user.ID, time.Now().Format(time.RFC3339)),
			}

			jsonData, err := json.Marshal(msg)
			if err != nil {
				stats.recordError()
				log.Printf("Error marshaling message: %v", err)
				continue
			}

			req, err := http.NewRequest("POST", BASE_URL+"/api/conversations/messages", bytes.NewBuffer(jsonData))
			if err != nil {
				stats.recordError()
				log.Printf("Error creating request: %v", err)
				continue
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+user.Token)

			start := time.Now()
			resp, err := client.Do(req)
			duration := time.Since(start)

			if err != nil {
				stats.recordError()
				log.Printf("Error sending message: %v", err)
				continue
			}

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				stats.recordError()
				log.Printf("Error response: %d", resp.StatusCode)
			} else {
				stats.recordSuccess(duration, WriteOperation)
			}

			resp.Body.Close()
		} else {
			// Read messages (read operation)
			conversationID := int64(rand.Intn(CONVERSATIONS) + 1)
			req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/conversations/messages?conversation_id=%d", BASE_URL, conversationID), nil)
			if err != nil {
				stats.recordError()
				log.Printf("Error creating request: %v", err)
				continue
			}

			req.Header.Set("Authorization", "Bearer "+user.Token)

			start := time.Now()
			resp, err := client.Do(req)
			duration := time.Since(start)

			if err != nil {
				stats.recordError()
				log.Printf("Error reading messages: %v", err)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				stats.recordError()
				log.Printf("Error response: %d", resp.StatusCode)
			} else {
				stats.recordSuccess(duration, ReadOperation)
			}

			resp.Body.Close()
		}
	}
}

func createUsersInParallel(start, end int, users []*User, wg *sync.WaitGroup, errChan chan<- error) {
	defer wg.Done()

	for i := start; i < end; i++ {
		user, err := registerUser(i)
		if err != nil {
			errChan <- fmt.Errorf("failed to register user %d: %v", i, err)
			continue
		}
		users[i] = user
	}
}

func createConversationsInParallel(adminUser *User) error {
	var wg sync.WaitGroup
	errChan := make(chan error, CONVERSATIONS)

	// Create conversations in batches
	batchSize := 10
	for i := 0; i < CONVERSATIONS; i += batchSize {
		end := i + batchSize
		if end > CONVERSATIONS {
			end = CONVERSATIONS
		}

		for j := i; j < end; j++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				if err := createConversation(id, adminUser); err != nil {
					errChan <- fmt.Errorf("failed to create conversation %d: %v", id, err)
				}
			}(j)
		}
		wg.Wait() // Wait for each batch to complete before starting the next
	}

	// Check for any errors
	close(errChan)
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to create some conversations: %v", errors)
	}

	return nil
}

func main() {
	log.Printf("Starting load test with %d users, %d messages per second per user, for %d seconds",
		NUM_USERS, MESSAGES_PER_SEC, SIMULATION_TIME)
	
	log.Printf("IMPORTANT: Make sure to start the server with the -loadtest flag:")
	log.Printf("  go run cmd/server/main.go -loadtest")
	log.Printf("This will use a separate database for load testing.\n")

	// Register admin user first
	adminUser, err := registerUser(-1) // special ID for admin
	if err != nil {
		log.Fatalf("Failed to register admin user: %v", err)
	}
	log.Printf("Admin user registered successfully")

	// Create conversations in parallel
	log.Printf("Creating %d conversations in parallel...", CONVERSATIONS)
	if err := createConversationsInParallel(adminUser); err != nil {
		log.Printf("Warning: %v", err)
	}
	log.Printf("Finished creating conversations")

	// Register users in parallel batches
	users := make([]*User, NUM_USERS)
	var wg sync.WaitGroup
	errChan := make(chan error, NUM_USERS)

	log.Printf("Creating %d users in parallel batches of %d...", NUM_USERS, BATCH_SIZE)
	startTime := time.Now()

	for i := 0; i < NUM_USERS; i += BATCH_SIZE {
		end := i + BATCH_SIZE
		if end > NUM_USERS {
			end = NUM_USERS
		}

		wg.Add(1)
		go createUsersInParallel(i, end, users, &wg, errChan)
	}

	// Wait for all user registrations to complete
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Process any errors while waiting
	errorCount := 0
	for err := range errChan {
		errorCount++
		if errorCount <= 10 { // Only log first 10 errors to avoid spam
			log.Printf("Error: %v", err)
		}
	}

	registrationDuration := time.Since(startTime)
	log.Printf("User registration completed in %v (%.2f users/sec)", 
		registrationDuration, 
		float64(NUM_USERS)/registrationDuration.Seconds())

	if errorCount > 0 {
		log.Printf("Warning: %d users failed to register", errorCount)
	}

	// Count successful registrations
	successfulUsers := 0
	for _, user := range users {
		if user != nil {
			successfulUsers++
		}
	}
	log.Printf("Successfully registered %d/%d users", successfulUsers, NUM_USERS)

	// Proceed with load test only if we have enough users
	if successfulUsers < NUM_USERS/2 {
		log.Fatalf("Too many registration failures, aborting load test")
	}

	// Start the actual load test
	var loadTestWg sync.WaitGroup
	stats := &Stats{
		writeLatencies: make([]time.Duration, 0, NUM_USERS*MESSAGES_PER_SEC*SIMULATION_TIME/2),
		readLatencies:  make([]time.Duration, 0, NUM_USERS*MESSAGES_PER_SEC*SIMULATION_TIME/2),
	}

	start := time.Now()

	// Start user simulations
	for _, user := range users {
		if user != nil {
			loadTestWg.Add(1)
			go simulateUser(user, &loadTestWg, stats)
		}
	}

	// Wait for all simulations to complete
	loadTestWg.Wait()
	duration := time.Since(start)

	// Calculate final stats
	stats.calculateStats(duration)

	// Print results
	log.Printf("\nLoad Test Results:")
	log.Printf("Total Requests: %d", stats.totalRequests)
	log.Printf("Successful Requests: %d", stats.successRequests)
	log.Printf("Failed Requests: %d", stats.failedRequests)
	log.Printf("Average Latency: %v", stats.totalLatency/time.Duration(stats.successRequests))
	log.Printf("Min Latency: %v", stats.minLatency)
	log.Printf("Max Latency: %v", stats.maxLatency)
	log.Printf("P99 Write Latency: %v", stats.getP99WriteLatency())
	log.Printf("P99 Read Latency: %v", stats.getP99ReadLatency())
	log.Printf("Requests per Second: %.2f", stats.requestsPerSecond)
	log.Printf("Total Duration: %v", duration)
} 