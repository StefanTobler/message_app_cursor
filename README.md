# Real-Time Messaging Application

A modern real-time messaging application built with Next.js, Go, and WebSocket. Features include direct messaging, group chats, and real-time updates.

## Features

- 🔐 User authentication (register/login)
- 💬 Real-time messaging using WebSocket
- 👥 Support for both direct and group conversations
- 📱 Responsive modern UI using shadcn/ui components
- 🔄 Automatic reconnection for WebSocket
- 💾 SQLite database for data persistence

## Prerequisites

- Node.js (v18 or later)
- Go (v1.21 or later)
- SQLite3

## Project Structure

```
.
├── src/                    # Frontend source code
│   ├── app/               # Next.js app directory
│   ├── components/        # React components
│   ├── lib/              # Utilities and API client
│   └── context/          # React context providers
├── backend/               # Go backend
│   ├── cmd/              # Entry points
│   └── internal/         # Internal packages
│       ├── api/         # HTTP handlers
│       ├── db/          # Database operations
│       ├── models/      # Data models
│       ├── config/      # Configuration
│       └── websocket/   # WebSocket handling
└── public/               # Static files
```

## Installation

1. Clone the repository:
\`\`\`bash
git clone <repository-url>
cd messaging-app
\`\`\`

2. Install frontend dependencies:
\`\`\`bash
npm install
\`\`\`

3. Install backend dependencies:
\`\`\`bash
cd backend
go mod download
\`\`\`

## Configuration

### Frontend
Create a \`.env.local\` file in the root directory:
\`\`\`env
NEXT_PUBLIC_API_URL=http://localhost:8080
\`\`\`

### Backend
The backend uses environment variables with sensible defaults:
- \`SERVER_ADDRESS\`: ":8080"
- \`DATABASE_URL\`: "sqlite://data/messenger.db"
- \`JWT_SECRET\`: "your-secret-key"

You can override these by setting environment variables.

## Running the Application

1. Start the backend server:
\`\`\`bash
cd backend
go run cmd/server/main.go
\`\`\`

2. In a new terminal, start the frontend development server:
\`\`\`bash
npm run dev
\`\`\`

The application will be available at:
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080

## API Endpoints

### Authentication
- \`POST /api/auth/register\`: Register a new user
- \`POST /api/auth/login\`: Login and receive JWT token

### Conversations
- \`GET /api/conversations\`: List user's conversations
- \`POST /api/conversations/create\`: Create a new conversation
- \`GET /api/conversations/messages\`: Get messages for a conversation

### WebSocket
- \`WS /ws\`: WebSocket endpoint for real-time messaging

## Database Schema

### Users
\`\`\`sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    avatar TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
\`\`\`

### Conversations
\`\`\`sql
CREATE TABLE conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
\`\`\`

### Messages
\`\`\`sql
CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER,
    sender_id INTEGER,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id),
    FOREIGN KEY (sender_id) REFERENCES users(id)
);
\`\`\`

## Security Considerations

- All API endpoints (except login/register) require JWT authentication
- Passwords are hashed using bcrypt
- WebSocket connections require authentication
- CORS is configured for development
- SQL injection protection through prepared statements

## Development

### Adding New Features
1. Backend: Add new endpoints in \`internal/api/handlers.go\`
2. Frontend: Add new API methods in \`src/lib/api.ts\`
3. Update components in \`src/components\`

### Testing WebSocket
You can test WebSocket connections using tools like [websocat](https://github.com/vi/websocat):
\`\`\`bash
websocat ws://localhost:8080/ws -H "Authorization: <your-jwt-token>"
\`\`\`

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a new Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
