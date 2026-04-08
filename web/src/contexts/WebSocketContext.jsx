import { createContext, useContext, useEffect, useState, useRef, useCallback } from 'react'
import api from '../api'
import { useAuth } from '../contexts/AuthContext'

const WebSocketContext = createContext(null)

export function WebSocketProvider({ children }) {
  const { isAuthenticated, user } = useAuth()
  const [socket, setSocket] = useState(null)
  const [connected, setConnected] = useState(false)
  const [messages, setMessages] = useState({})
  const [rooms, setRooms] = useState(new Set())
  // 待确认消息 { tempId: { conversationId, content, sender_id } }
  const [pendingMessages, setPendingMessages] = useState({})
  const reconnectTimerRef = useRef(null)
  const socketRef = useRef(null)

  const connect = useCallback(() => {
    if (!isAuthenticated || !user) return

    const token = api.getToken()
    if (!token) return

    const wsUrl = `ws://localhost:8080/api/v1/ws/connect?token=${token}&device=PC`
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      console.log('WebSocket connected')
      setConnected(true)
      setSocket(ws)
      socketRef.current = ws

      if (rooms.size > 0) {
        rooms.forEach(roomId => {
          joinRoom(ws, roomId)
        })
      }
    }

    ws.onmessage = (event) => {
      try {
        const rawData = JSON.parse(event.data)
        // 处理数组格式（历史消息）和单个消息
        const dataArray = Array.isArray(rawData) ? rawData : [rawData]
        dataArray.forEach(data => handleMessage(data))
      } catch (err) {
        console.error('Failed to parse WebSocket message:', err)
      }
    }

    ws.onclose = () => {
      console.log('WebSocket disconnected')
      setConnected(false)
      setSocket(null)
      socketRef.current = null

      reconnectTimerRef.current = setTimeout(() => {
        if (isAuthenticated) {
          connect()
        }
      }, 3000)
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }
  }, [isAuthenticated, user, rooms])

  const handleMessage = (data) => {
    // 忽略确认消息（因为我们用内容匹配，不依赖msg_tag）
    if (data.type === 'send_confirmation') {
      return
    }

    const conversationId = data.conversation_id
    if (!conversationId) return

    const currentUserId = user?.id

    // 添加消息到列表（替换 pending 消息或避免重复）
    setMessages(prev => {
      const convMessages = prev[conversationId] || []
      
      // 如果有 pending 消息，通过 content + sender_id 匹配找到并替换
      if (data.id && data.sender_id === currentUserId && data.content) {
        const pendingIndex = convMessages.findIndex(m => 
          m.status === 'pending' && 
          m.content === data.content && 
          m.sender_id === currentUserId
        )
        
        if (pendingIndex !== -1) {
          // 替换 pending 消息
          const newMessages = [...convMessages]
          newMessages[pendingIndex] = data
          console.log('替换pending消息:', data.id, '内容:', data.content)
          
          // 移除对应的 pending 消息
          setPendingMessages(prevPending => {
            const newPending = { ...prevPending }
            for (const key of Object.keys(newPending)) {
              const pending = newPending[key]
              if (pending.conversationId === conversationId && 
                  pending.content === data.content && 
                  pending.sender_id === currentUserId) {
                delete newPending[key]
                break
              }
            }
            return newPending
          })
          
          return { ...prev, [conversationId]: newMessages }
        }
      }
      
      // 检查是否重复
      const exists = convMessages.some(m => m.id === data.id)
      if (exists) {
        return prev
      }
      
      return { ...prev, [conversationId]: [...convMessages, data] }
    })
  }

  const joinRoom = (ws, roomId) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        action: 'join_room',
        room_id: parseInt(roomId)
      }))
    }
  }

  const leaveRoom = (roomId) => {
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({
        action: 'leave_room',
        room_id: parseInt(roomId)
      }))
    }
    setRooms(prev => {
      const newRooms = new Set(prev)
      newRooms.delete(roomId)
      return newRooms
    })
  }

  const sendMessage = (conversationId, content, msgType = 1) => {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      console.error('WebSocket not connected')
      return false
    }

    const conversationIdInt = parseInt(conversationId)
    console.log('WS发送消息 - 会话ID:', conversationIdInt, '内容:', content)
    
    const tempId = `temp_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`

    const message = {
      conversation_id: conversationIdInt,
      msg_type: msgType,
      content: content,
      sender_id: user?.id
    }

    socket.send(JSON.stringify(message))

    // 添加到待确认消息（用 tempId 作为 key，便于后续匹配）
    const pendingInfo = { conversationId: conversationIdInt, content, sender_id: user?.id }
    setPendingMessages(prev => ({
      ...prev,
      [tempId]: pendingInfo
    }))

    // 同时添加到消息列表显示（待确认状态）
    setMessages(prev => {
      const convMessages = prev[conversationIdInt] || []
      return {
        ...prev,
        [conversationIdInt]: [...convMessages, {
          temp_id: tempId,
          content: content,
          sender_id: user?.id,
          status: 'pending',
          conversation_id: conversationIdInt,
          msg_type: msgType,
          send_time: new Date().toISOString()
        }]
      }
    })

    return true
  }

  const joinRoomById = (roomId) => {
    setRooms(prev => {
      if (prev.has(roomId)) return prev
      const newRooms = new Set(prev)
      newRooms.add(roomId)
      return newRooms
    })

    if (socket && socket.readyState === WebSocket.OPEN) {
      joinRoom(socket, roomId)
    }
  }

  const clearMessages = (conversationId) => {
    setMessages(prev => {
      const newMessages = { ...prev }
      delete newMessages[conversationId]
      return newMessages
    })
  }

  useEffect(() => {
    if (isAuthenticated && user) {
      connect()
    }

    return () => {
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
      }
      if (socket) {
        socket.close()
      }
    }
  }, [isAuthenticated, user])

  useEffect(() => {
    if (socket && rooms.size > 0) {
      rooms.forEach(roomId => {
        joinRoom(socket, roomId)
      })
    }
  }, [socket, rooms])

  return (
    <WebSocketContext.Provider value={{
      socket,
      connected,
      messages,
      rooms,
      pendingMessages,
      joinRoomById,
      leaveRoom,
      sendMessage,
      clearMessages
    }}>
      {children}
    </WebSocketContext.Provider>
  )
}

export function useWebSocket() {
  const context = useContext(WebSocketContext)
  if (!context) {
    throw new Error('useWebSocket must be used within a WebSocketProvider')
  }
  return context
}