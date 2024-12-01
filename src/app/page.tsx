"use client"

import { useState } from "react"
import MessageList from "@/components/message-list"
import ChatView from "@/components/chat-view"
import { ChatProvider } from "@/context/chat-context"
import { Button } from "@/components/ui/button"
import { Menu, X, Loader2 } from "lucide-react"
import { AuthForm } from "@/components/ui/auth-form"
import { useChat } from "@/context/chat-context"

function LoadingScreen() {
  return (
    <div className="h-screen flex items-center justify-center">
      <div className="flex flex-col items-center gap-4">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
        <p className="text-sm text-muted-foreground">Loading your messages...</p>
      </div>
    </div>
  )
}

function MessagesApp() {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const { user, initialLoading } = useChat()

  if (initialLoading) {
    return <LoadingScreen />
  }

  if (!user) {
    return (
      <div className="h-screen flex items-center justify-center p-4">
        <AuthForm />
      </div>
    )
  }

  return (
    <main className="flex h-screen relative">
      {/* Mobile menu button */}
      <Button
        variant="ghost"
        size="icon"
        className="absolute top-2 left-2 z-50 md:hidden"
        onClick={() => setSidebarOpen(!sidebarOpen)}
      >
        {sidebarOpen ? <X size={24} /> : <Menu size={24} />}
      </Button>

      {/* Sidebar */}
      <div
        className={`${
          sidebarOpen ? "translate-x-0" : "-translate-x-full"
        } md:translate-x-0 transform transition-transform duration-200 ease-in-out fixed md:relative z-40 w-full md:w-1/3 lg:w-1/4 h-full bg-background border-r`}
      >
        <MessageList onSelectThread={() => setSidebarOpen(false)} />
      </div>

      {/* Main content */}
      <div className="flex-1 w-full md:w-2/3 lg:w-3/4">
        <ChatView />
      </div>

      {/* Overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/20 z-30 md:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}
    </main>
  )
}

export default function Home() {
  return (
    <ChatProvider>
      <MessagesApp />
    </ChatProvider>
  )
}
