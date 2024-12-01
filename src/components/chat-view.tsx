"use client";

import { useState, useRef, useEffect } from "react";
import { Card } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { useChat } from "@/context/chat-context";
import { Loader2, Send, Bell, BellOff } from "lucide-react";

export default function ChatView() {
  const {
    selectedThread,
    messages = [],
    sendMessage,
    user,
    loading,
    connected,
  } = useChat();
  const [newMessage, setNewMessage] = useState("");
  const [sending, setSending] = useState(false);
  const scrollAreaRef = useRef<HTMLDivElement>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [notificationsEnabled, setNotificationsEnabled] = useState<boolean>(
    () => {
      // Load notification preference from localStorage
      const stored = localStorage.getItem(
        `notifications_${selectedThread?.id}`
      );
      return stored ? JSON.parse(stored) : true;
    }
  );

  // Sort messages by creation time
  const sortedMessages = [...messages].sort(
    (a, b) =>
      new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
  );

  // Request notification permission on mount
  useEffect(() => {
    if ("Notification" in window) {
      Notification.requestPermission();
    }
  }, []);

  // Update notification preference when thread changes
  useEffect(() => {
    if (selectedThread) {
      const stored = localStorage.getItem(`notifications_${selectedThread.id}`);
      setNotificationsEnabled(stored ? JSON.parse(stored) : true);
    }
  }, [selectedThread]);

  // Handle notifications for new messages
  useEffect(() => {
    const lastMessage = sortedMessages[sortedMessages.length - 1];
    if (
      lastMessage &&
      lastMessage.sender_id !== user?.id &&
      notificationsEnabled &&
      document.hidden &&
      "Notification" in window &&
      Notification.permission === "granted"
    ) {
      new Notification(`New message from ${selectedThread?.name}`, {
        body: lastMessage.content,
        icon: "/app-icon.png", // Add your app icon path here
      });
    }
  }, [sortedMessages, user?.id, selectedThread?.name, notificationsEnabled]);

  const toggleNotifications = () => {
    if (!selectedThread) return;

    const newValue = !notificationsEnabled;
    setNotificationsEnabled(newValue);
    localStorage.setItem(
      `notifications_${selectedThread.id}`,
      JSON.stringify(newValue)
    );
  };

  const scrollToBottom = () => {
    if (scrollAreaRef.current) {
      const scrollContainer = scrollAreaRef.current.querySelector(
        "[data-radix-scroll-area-viewport]"
      );
      if (scrollContainer) {
        scrollContainer.scrollTop = scrollContainer.scrollHeight;
      }
    }
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  useEffect(() => {
    if (newMessage === "" && !sending && connected) {
      const timeoutId = setTimeout(() => {
        textareaRef.current?.focus();
      }, 0);
      return () => clearTimeout(timeoutId);
    }
  }, [newMessage, sending, connected]);

  useEffect(() => {
    if (selectedThread && connected && !sending) {
      const timeoutId = setTimeout(() => {
        textareaRef.current?.focus();
      }, 0);
      return () => clearTimeout(timeoutId);
    }
  }, [selectedThread, connected, sending]);

  const handleSendMessage = async () => {
    if (!newMessage.trim() || sending || !connected) return;
    setSending(true);
    try {
      await sendMessage(newMessage);
      setNewMessage("");
    } catch (error) {
      console.error("Failed to send message:", error);
    } finally {
      setSending(false);
      setTimeout(() => {
        textareaRef.current?.focus();
      }, 0);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
  };

  if (!selectedThread) {
    return (
      <div className="h-full flex items-center justify-center text-gray-500 p-4 text-center">
        <div>
          <h3 className="text-lg font-medium mb-2">Welcome to Messages</h3>
          <p>Select a conversation to start messaging</p>
        </div>
      </div>
    );
  }

  return (
    <Card className="h-full flex flex-col">
      <div className="p-4 border-b flex items-center gap-3">
        <Avatar className="h-10 w-10">
          <AvatarFallback>{selectedThread.name[0]}</AvatarFallback>
        </Avatar>
        <div className="flex-1">
          <h2 className="font-semibold">{selectedThread.name}</h2>
          <p className="text-sm text-gray-500">
            {selectedThread.type === "direct" ? "Direct Message" : "Group"}
          </p>
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={toggleNotifications}
          title={
            notificationsEnabled
              ? "Disable notifications"
              : "Enable notifications"
          }
          className="text-gray-500 hover:text-gray-700"
        >
          {notificationsEnabled ? (
            <Bell className="h-4 w-4" />
          ) : (
            <BellOff className="h-4 w-4" />
          )}
        </Button>
        {!connected && (
          <div className="flex items-center gap-2 text-yellow-600">
            <Loader2 className="h-4 w-4 animate-spin" />
            <span className="text-sm">Connecting...</span>
          </div>
        )}
      </div>

      <ScrollArea ref={scrollAreaRef} className="flex-1 p-4">
        {sortedMessages.length > 0 ? (
          <div className="flex flex-col justify-start min-h-full">
            <div className="space-y-4">
              {sortedMessages.map((message) => (
                <div
                  key={message.id}
                  className={`flex flex-col ${
                    message.sender_id === user?.id ? "items-end" : "items-start"
                  }`}
                >
                  <div
                    className={`max-w-[85%] md:max-w-[70%] rounded-lg p-3 ${
                      message.sender_id === user?.id
                        ? "bg-blue-500 text-white"
                        : "bg-gray-100"
                    }`}
                  >
                    <p className="whitespace-pre-wrap break-words">
                      {message.content}
                    </p>
                    <span className="text-xs opacity-70 mt-1 block">
                      {new Date(message.created_at).toLocaleTimeString([], {
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </span>
                  </div>
                </div>
              ))}
              <div ref={messagesEndRef} />
            </div>
          </div>
        ) : (
          <div className="h-full flex items-center justify-center text-gray-500">
            <p>No messages yet. Start the conversation!</p>
          </div>
        )}
      </ScrollArea>

      <div className="p-4 border-t flex gap-2">
        <Textarea
          ref={textareaRef}
          value={newMessage}
          onChange={(e) => setNewMessage(e.target.value)}
          onKeyDown={handleKeyPress}
          placeholder={connected ? "Type a message..." : "Connecting..."}
          className="resize-none min-h-[2.5rem] max-h-32"
          rows={1}
          disabled={sending || !connected}
          autoFocus
        />
        <Button
          onClick={handleSendMessage}
          disabled={!newMessage.trim() || sending || !connected}
          className="shrink-0"
        >
          {sending ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Send className="h-4 w-4" />
          )}
          <span className="ml-2 hidden md:inline">Send</span>
        </Button>
      </div>
    </Card>
  );
}
