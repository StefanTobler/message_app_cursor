"use client"

import { useState } from "react"
import { Card } from "./card"
import { Input } from "./input"
import { Button } from "./button"
import { useChat } from "@/context/chat-context"
import { Loader2 } from "lucide-react"

export function AuthForm() {
  const { login, register, loading } = useChat()
  const [isLogin, setIsLogin] = useState(true)
  const [formData, setFormData] = useState({
    username: "",
    password: "",
    avatar: "",
  })

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      if (isLogin) {
        await login(formData.username, formData.password)
      } else {
        await register(formData.username, formData.password, formData.avatar)
      }
    } catch (error) {
      console.error("Authentication failed:", error)
    }
  }

  return (
    <Card className="w-full max-w-md p-6 space-y-4">
      <div className="space-y-2 text-center">
        <h1 className="text-2xl font-bold">Welcome to Messages</h1>
        <p className="text-gray-500">
          {isLogin ? "Sign in to continue" : "Create a new account"}
        </p>
      </div>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="space-y-2">
          <label className="text-sm font-medium" htmlFor="username">
            Username
          </label>
          <Input
            id="username"
            required
            disabled={loading}
            value={formData.username}
            onChange={(e) =>
              setFormData({ ...formData, username: e.target.value })
            }
          />
        </div>
        <div className="space-y-2">
          <label className="text-sm font-medium" htmlFor="password">
            Password
          </label>
          <Input
            id="password"
            type="password"
            required
            disabled={loading}
            value={formData.password}
            onChange={(e) =>
              setFormData({ ...formData, password: e.target.value })
            }
          />
        </div>
        {!isLogin && (
          <div className="space-y-2">
            <label className="text-sm font-medium" htmlFor="avatar">
              Avatar URL (optional)
            </label>
            <Input
              id="avatar"
              type="url"
              disabled={loading}
              value={formData.avatar}
              onChange={(e) =>
                setFormData({ ...formData, avatar: e.target.value })
              }
            />
          </div>
        )}
        <Button className="w-full" type="submit" disabled={loading}>
          {loading ? (
            <>
              <Loader2 className="h-4 w-4 animate-spin mr-2" />
              {isLogin ? "Signing in..." : "Creating account..."}
            </>
          ) : (
            <>{isLogin ? "Sign In" : "Sign Up"}</>
          )}
        </Button>
      </form>
      <div className="text-center">
        <Button
          variant="link"
          onClick={() => setIsLogin(!isLogin)}
          className="text-sm"
          disabled={loading}
        >
          {isLogin
            ? "Don't have an account? Sign up"
            : "Already have an account? Sign in"}
        </Button>
      </div>
    </Card>
  )
} 