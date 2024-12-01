const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export interface User {
  id: number;
  username: string;
  avatar: string;
  created_at: string;
}

export interface Message {
  id: number;
  conversation_id: number;
  sender_id: number;
  content: string;
  created_at: string;
}

export interface Conversation {
  id: number;
  name: string;
  type: string;
  created_at: string;
}

class ApiClient {
  private async fetch(endpoint: string, options: RequestInit = {}) {
    const headers = {
      'Content-Type': 'application/json',
      ...options.headers,
    };

    try {
      const response = await fetch(`${API_URL}${endpoint}`, {
        ...options,
        headers,
        credentials: 'include',
      });

      if (!response.ok) {
        const error = await response.text();
        throw new Error(error || response.statusText);
      }

      return response.json();
    } catch (error) {
      console.error(`API Error (${endpoint}):`, error);
      throw error;
    }
  }

  async verifyToken(): Promise<User> {
    return this.fetch('/api/auth/verify');
  }

  async register(username: string, password: string, avatar: string) {
    return this.fetch('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username, password, avatar }),
    });
  }

  async login(username: string, password: string) {
    return this.fetch('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
  }

  async logout() {
    return this.fetch('/api/auth/logout', {
      method: 'POST',
    });
  }

  async getConversations(): Promise<Conversation[]> {
    return this.fetch('/api/conversations');
  }

  async createConversation(name: string, type: string, participants: number[]) {
    return this.fetch('/api/conversations/create', {
      method: 'POST',
      body: JSON.stringify({ name, type, participants }),
    });
  }

  async getMessages(conversationId: number, offset = 0): Promise<Message[]> {
    return this.fetch(`/api/conversations/messages?conversation_id=${conversationId}&offset=${offset}`);
  }

  connectWebSocket(onMessage: (data: any) => void): WebSocket {
    const wsUrl = API_URL.replace('http://', 'ws://').replace('https://', 'wss://');
    const ws = new WebSocket(`${wsUrl}/ws`);

    ws.onopen = () => {
      console.log('WebSocket connected');
    };
    
    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        onMessage(data);
      } catch (error) {
        console.error('Error parsing WebSocket message:', error);
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
      setTimeout(() => this.connectWebSocket(onMessage), 5000);
    };

    ws.onclose = () => {
      console.log('WebSocket connection closed');
      setTimeout(() => this.connectWebSocket(onMessage), 5000);
    };

    return ws;
  }

  async searchUsers(query: string): Promise<Array<{ id: number; username: string; avatar: string }>> {
    return this.fetch(`/api/users?search=${encodeURIComponent(query)}`);
  }
}

export const api = new ApiClient(); 