"use client";

import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  ReactNode,
} from "react";
import { api } from "@/lib/api";

interface User {
  id: number;
  username: string;
  avatar: string;
}

interface Message {
  id: number;
  conversation_id: number;
  sender_id: number;
  content: string;
  created_at: string;
}

interface Conversation {
  id: number;
  name: string;
  type: "direct" | "group";
}

interface ChatContextType {
  user: User | null;
  messages: Message[];
  conversations: Conversation[];
  selectedThread: Conversation | null;
  loading: boolean;
  initialLoading: boolean;
  connected: boolean;
  setSelectedThread: (thread: Conversation | null) => void;
  sendMessage: (content: string) => Promise<void>;
  login: (username: string, password: string) => Promise<void>;
  register: (
    username: string,
    password: string,
    avatar: string
  ) => Promise<void>;
  logout: () => Promise<void>;
  refreshConversations: () => Promise<Conversation[]>;
  addNewConversation: (conversation: Conversation) => Promise<void>;
}

const ChatContext = createContext<ChatContextType | undefined>(undefined);

// Save and load state from localStorage
const saveState = (key: string, value: any) => {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch (error) {
    console.error("Error saving state:", error);
  }
};

const loadState = <T,>(key: string, defaultValue: T): T => {
  try {
    const item = localStorage.getItem(key);
    return item ? JSON.parse(item) : defaultValue;
  } catch (error) {
    console.error("Error loading state:", error);
    return defaultValue;
  }
};

export function ChatProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [selectedThread, setSelectedThread] = useState<Conversation | null>(
    loadState("selectedThread", null)
  );
  const [loading, setLoading] = useState(false);
  const [initialLoading, setInitialLoading] = useState(true);
  const [socket, setSocket] = useState<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);

  // Check auth status on mount
  useEffect(() => {
    const checkAuth = async () => {
      try {
        const userData = await api.verifyToken();
        setUser(userData);
      } catch (error) {
        console.error("Auth check failed:", error);
        setUser(null);
      } finally {
        setInitialLoading(false);
      }
    };
    checkAuth();
  }, []);

  // Save selected thread to localStorage
  useEffect(() => {
    if (selectedThread) {
      saveState("selectedThread", selectedThread);
    }
  }, [selectedThread]);

  // Load conversations when user is authenticated
  useEffect(() => {
    const fetchConversations = async () => {
      if (!user) {
        setConversations([]);
        return;
      }

      try {
        setLoading(true);
        const convos = await api.getConversations();
        setConversations(convos);

        // If there's a stored thread, verify it exists in conversations
        if (selectedThread) {
          const threadExists = convos.some((c) => c.id === selectedThread.id);
          if (!threadExists) {
            setSelectedThread(null);
            saveState("selectedThread", null);
          }
        }
      } catch (error) {
        console.error("Failed to fetch conversations:", error);
        setConversations([]);
      } finally {
        setLoading(false);
      }
    };
    fetchConversations();
  }, [user, selectedThread]);

  // Fetch messages when thread changes
  useEffect(() => {
    const fetchMessages = async () => {
      if (!selectedThread) {
        setMessages([]);
        return;
      }

      setLoading(true);
      try {
        const msgs = await api.getMessages(selectedThread.id);
        setMessages(msgs || []);
      } catch (error) {
        console.error("Failed to fetch messages:", error);
        setMessages([]);
      } finally {
        setLoading(false);
      }
    };
    fetchMessages();
  }, [selectedThread]);

  // WebSocket connection
  useEffect(() => {
    if (!user) {
      setConnected(false);
      return;
    }

    try {
      const ws = api.connectWebSocket((data) => {
        if (
          data.type === "message" &&
          data.payload.conversation_id === selectedThread?.id
        ) {
          setMessages((prev) => [...prev, data.payload]);
        }
      });

      ws.onopen = () => {
        console.log("WebSocket connected");
        setConnected(true);
      };

      ws.onclose = () => {
        console.log("WebSocket disconnected");
        setConnected(false);
      };

      ws.onerror = (error) => {
        console.error("WebSocket error:", error);
        setConnected(false);
      };

      setSocket(ws);

      return () => {
        ws.close();
        setSocket(null);
        setConnected(false);
      };
    } catch (error) {
      console.error("WebSocket connection failed:", error);
      setConnected(false);
    }
  }, [user, selectedThread?.id]);

  const sendMessage = useCallback(
    async (content: string) => {
      if (!selectedThread || !user || !socket || !connected) {
        throw new Error("Cannot send message: not connected");
      }

      try {
        socket.send(
          JSON.stringify({
            type: "message",
            payload: {
              conversation_id: selectedThread.id,
              content,
            },
          })
        );
      } catch (error) {
        console.error("Failed to send message:", error);
        throw error;
      }
    },
    [selectedThread, user, socket, connected]
  );

  const login = async (username: string, password: string) => {
    setLoading(true);
    try {
      const response = await api.login(username, password);
      setUser(response.user);
    } catch (error) {
      console.error("Login failed:", error);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const register = async (
    username: string,
    password: string,
    avatar: string
  ) => {
    setLoading(true);
    try {
      await api.register(username, password, avatar);
      await login(username, password);
    } catch (error) {
      console.error("Registration failed:", error);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const logout = async () => {
    try {
      await api.logout();
      setUser(null);
      setConversations([]);
      setMessages([]);
      setSelectedThread(null);
      socket?.close();
      setSocket(null);
      saveState("selectedThread", null);
    } catch (error) {
      console.error("Logout failed:", error);
      throw error;
    }
  };

  const refreshConversations = useCallback(async () => {
    if (!user) {
      setConversations([]);
      return;
    }

    try {
      const convos = await api.getConversations();
      setConversations(convos);
      return convos;
    } catch (error) {
      console.error("Failed to fetch conversations:", error);
      return [];
    }
  }, [user]);

  const addNewConversation = useCallback(async (conversation: Conversation) => {
    // Add the new conversation to the list
    setConversations((prev) => {
      // Check if conversation already exists
      const exists = prev.some((c) => c.id === conversation.id);
      if (exists) {
        return prev;
      }
      return [conversation, ...prev];
    });

    // Select the new conversation
    setSelectedThread(conversation);
    saveState("selectedThread", conversation);
  }, []);

  return (
    <ChatContext.Provider
      value={{
        user,
        messages,
        conversations,
        selectedThread,
        loading,
        initialLoading,
        connected,
        setSelectedThread,
        sendMessage,
        login,
        register,
        logout,
        refreshConversations,
        addNewConversation,
      }}
    >
      {children}
    </ChatContext.Provider>
  );
}

export function useChat() {
  const context = useContext(ChatContext);
  if (context === undefined) {
    throw new Error("useChat must be used within a ChatProvider");
  }
  return context;
}
