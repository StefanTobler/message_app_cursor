"use client"

import { ScrollArea } from "@/components/ui/scroll-area"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { useChat } from "@/context/chat-context"
import { Input } from "@/components/ui/input"
import { useState, useEffect } from "react"
import { Search, Loader2, Plus, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { api } from "@/lib/api"

interface MessageListProps {
  onSelectThread?: () => void
}

export default function MessageList({ onSelectThread }: MessageListProps) {
  const { 
    selectedThread, 
    setSelectedThread, 
    conversations, 
    loading,
    addNewConversation 
  } = useChat()
  
  const [searchQuery, setSearchQuery] = useState("")
  const [showNewMessage, setShowNewMessage] = useState(false)
  const [userSearchQuery, setUserSearchQuery] = useState("")
  const [searchResults, setSearchResults] = useState<Array<{ id: number; username: string }>>([])
  const [searchLoading, setSearchLoading] = useState(false)
  const [selectedIndex, setSelectedIndex] = useState(-1)

  const filteredConversations = (conversations || []).filter((conversation) =>
    conversation.name.toLowerCase().includes(searchQuery.toLowerCase())
  )

  // Reset selected index when search query changes
  useEffect(() => {
    setSelectedIndex(-1);
  }, [userSearchQuery]);

  // Debounced user search
  useEffect(() => {
    if (!userSearchQuery) {
      setSearchResults([]);
      return;
    }

    const timeoutId = setTimeout(async () => {
      setSearchLoading(true);
      try {
        const users = await api.searchUsers(userSearchQuery);
        setSearchResults(users);
      } catch (error) {
        console.error('Error searching users:', error);
      } finally {
        setSearchLoading(false);
      }
    }, 200); // Reduced debounce time for faster response

    return () => clearTimeout(timeoutId);
  }, [userSearchQuery]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!searchResults.length) return;

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex(prev => 
          prev < searchResults.length - 1 ? prev + 1 : prev
        );
        break;
      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex(prev => prev > 0 ? prev - 1 : prev);
        break;
      case 'Enter':
        e.preventDefault();
        if (selectedIndex >= 0 && selectedIndex < searchResults.length) {
          const selectedUser = searchResults[selectedIndex];
          startNewConversation(selectedUser.id, selectedUser.username);
        }
        break;
      case 'Escape':
        e.preventDefault();
        setShowNewMessage(false);
        setUserSearchQuery("");
        break;
    }
  };

  const startNewConversation = async (userId: number, username: string) => {
    try {
      // First check if a conversation already exists with this user
      const existingConversation = conversations.find(conv => 
        conv.type === "direct" && conv.name === username
      );

      if (existingConversation) {
        // If conversation exists, just select it
        setSelectedThread(existingConversation);
        setShowNewMessage(false);
        setUserSearchQuery("");
        onSelectThread?.();
        return;
      }

      // If no existing conversation, create a new one
      const response = await api.createConversation(username, "direct", [userId]);
      setShowNewMessage(false);
      setUserSearchQuery("");
      await addNewConversation(response);
      onSelectThread?.();
    } catch (error) {
      console.error('Error creating conversation:', error);
    }
  };

  return (
    <div className="h-full flex flex-col">
      <div className="p-4 border-b space-y-4">
        <div className="flex justify-between items-center">
          <h2 className="text-xl font-semibold">Messages</h2>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setShowNewMessage(true)}
            className="hover:bg-gray-100 rounded-full"
          >
            <Plus className="h-5 w-5" />
          </Button>
        </div>

        {showNewMessage ? (
          <div className="relative">
            <div className="flex items-center gap-2">
              <Input
                placeholder="Search users..."
                value={userSearchQuery}
                onChange={(e) => setUserSearchQuery(e.target.value)}
                onKeyDown={handleKeyDown}
                autoFocus
                className="flex-1"
              />
              <Button
                variant="ghost"
                size="icon"
                onClick={() => {
                  setShowNewMessage(false);
                  setUserSearchQuery("");
                }}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
            {userSearchQuery && (
              <div className="absolute z-10 left-0 right-0 mt-1 bg-white border rounded-md shadow-lg max-h-48 overflow-auto">
                {searchLoading ? (
                  <div className="p-2 text-center">
                    <Loader2 className="h-4 w-4 animate-spin inline" />
                  </div>
                ) : searchResults.length > 0 ? (
                  searchResults.map((user, index) => (
                    <button
                      key={user.id}
                      className={`w-full text-left px-3 py-2 hover:bg-gray-100 focus:bg-gray-100 focus:outline-none ${
                        index === selectedIndex ? 'bg-gray-100' : ''
                      }`}
                      onClick={() => startNewConversation(user.id, user.username)}
                      onMouseEnter={() => setSelectedIndex(index)}
                    >
                      <div className="flex items-center gap-2">
                        <Avatar className="h-6 w-6">
                          <AvatarFallback>{user.username[0]}</AvatarFallback>
                        </Avatar>
                        <span>{user.username}</span>
                      </div>
                    </button>
                  ))
                ) : (
                  <div className="p-2 text-center text-gray-500">No users found</div>
                )}
              </div>
            )}
          </div>
        ) : (
          <div className="relative">
            <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Search conversations..."
              className="pl-8"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </div>
        )}
      </div>

      <ScrollArea className="flex-1">
        {loading ? (
          <div className="h-full flex items-center justify-center">
            <Loader2 className="h-6 w-6 animate-spin text-gray-500" />
          </div>
        ) : filteredConversations.length > 0 ? (
          <div className="space-y-1 p-2">
            {filteredConversations.map((conversation) => (
              <div
                key={conversation.id}
                className={`flex items-center gap-3 p-3 rounded-lg cursor-pointer hover:bg-gray-100 ${
                  selectedThread?.id === conversation.id ? "bg-gray-100" : ""
                }`}
                onClick={() => {
                  setSelectedThread(conversation);
                  onSelectThread?.();
                }}
              >
                <Avatar className="h-12 w-12">
                  <AvatarFallback className="text-lg">
                    {conversation.name[0]}
                  </AvatarFallback>
                </Avatar>
                <div className="flex-1 min-w-0">
                  <p className="font-medium truncate">{conversation.name}</p>
                  <p className="text-sm text-gray-500 truncate">
                    {conversation.type === "direct" ? "Direct Message" : "Group"}
                  </p>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="h-full flex items-center justify-center text-gray-500 p-4 text-center">
            <p>No conversations found</p>
          </div>
        )}
      </ScrollArea>
    </div>
  )
} 