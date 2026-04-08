import { createContext, useContext, useState, useEffect } from 'react'
import api from '../api'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)
  const [isAuthenticated, setIsAuthenticated] = useState(false)

  useEffect(() => {
    const token = api.getToken()
    const savedUser = api.getUser()
    if (token && savedUser) {
      setUser(savedUser)
      setIsAuthenticated(true)
    }
    setLoading(false)
  }, [])

  const login = async (username, password, device = 'PC') => {
    const result = await api.post('/auth/login', { username, password, device })
    if (result.code === 200 && result.data) {
      api.setTokens(result.data.access_token, result.data.refresh_token)
      api.setUser(result.data.user)
      setUser(result.data.user)
      setIsAuthenticated(true)
      return { success: true }
    }
    return { success: false, message: result.message }
  }

  const register = async (username, password) => {
    const result = await api.post('/auth/register', { username, password })
    if (result.code === 200) {
      return { success: true }
    }
    return { success: false, message: result.message }
  }

  const logout = () => {
    api.clearTokens()
    setUser(null)
    setIsAuthenticated(false)
  }

  const updateUser = (userData) => {
    setUser(userData)
    api.setUser(userData)
  }

  return (
    <AuthContext.Provider value={{ user, loading, isAuthenticated, login, register, logout, updateUser }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}