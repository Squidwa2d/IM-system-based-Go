import { useState, useEffect, useRef } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'
import { useWebSocket } from '../contexts/WebSocketContext'
import api from '../api'

export default function Dashboard() {
  const { isAuthenticated, user, logout } = useAuth()
  const { connected, messages, joinRoomById, sendMessage, pendingMessages } = useWebSocket()
  
  const [conversations, setConversations] = useState([])
  const [friends, setFriends] = useState([])
  const [friendRequests, setFriendRequests] = useState([])
  const [selectedConversation, setSelectedConversation] = useState(null)
  const [messageInput, setMessageInput] = useState('')
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [uploadingAvatar, setUploadingAvatar] = useState(false)
  const fileInputRef = useRef(null)
  const avatarInputRef = useRef(null)
  
  // 添加好友相关状态
  const [showAddFriend, setShowAddFriend] = useState(false)
  const [searchKeyword, setSearchKeyword] = useState('')
  const [searchResults, setSearchResults] = useState([])
  const [searching, setSearching] = useState(false)
  const [showRequests, setShowRequests] = useState(false)
  
  const messagesEndRef = useRef(null)

  useEffect(() => {
    if (!isAuthenticated) return
    loadData()
  }, [isAuthenticated])

  useEffect(() => {
    if (selectedConversation) {
      const convId = getConversationId(selectedConversation)
      joinRoomById(convId)
    }
  }, [selectedConversation])

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, selectedConversation])

  const loadData = async () => {
    setLoading(true)
    try {
      const [convResult, friendResult, requestResult] = await Promise.all([
        api.getConversations(),
        api.getFriendList(),
        api.getFriendRequests()
      ])
      
      if (convResult.code === 200 && convResult.data) {
        setConversations(convResult.data.conversations || [])
      }
      if (friendResult.code === 200 && friendResult.data) {
        setFriends(friendResult.data.friends || [])
      }
      if (requestResult.code === 200 && requestResult.data) {
        setFriendRequests(requestResult.data.requests || [])
      }
    } catch (err) {
      console.error('加载数据失败:', err)
    } finally {
      setLoading(false)
    }
  }

  // 合并会话和好友为一个列表
  const getChatList = () => {
    const chatList = []
    
    // 添加会话（私聊和群聊）
    conversations.forEach(conv => {
      const convId = conv.ID || conv.id
      let name = conv.Name || conv.name || conv.target_username
      if (!name) {
        name = (conv.Type || conv.type) === 1 ? '私聊' : '群聊'
      }
      chatList.push({
        id: convId,
        name: name,
        type: (conv.Type || conv.type) === 1 ? 'private' : 'group',
        conv: conv
      })
    })
    
    return chatList
  }

  // 搜索用户
  const handleSearch = async () => {
    if (!searchKeyword.trim()) return
    setSearching(true)
    try {
      const result = await api.searchUsers(searchKeyword)
      if (result.code === 200 && result.data) {
        setSearchResults(result.data.users || [])
      }
    } catch (err) {
      console.error('搜索失败:', err)
    } finally {
      setSearching(false)
    }
  }

  // 添加好友
  const handleAddFriend = async (targetUsername) => {
    try {
      const result = await api.addFriend(targetUsername)
      if (result.code === 200) {
        alert('好友请求已发送')
        setSearchResults(prev => prev.filter(u => u.Username !== targetUsername))
      } else {
        alert(result.message || '添加好友失败')
      }
    } catch (err) {
      alert('添加好友失败')
    }
  }

  // 接受好友请求
  const handleAcceptFriend = async (requesterUsername) => {
    try {
      const result = await api.acceptFriendRequest(requesterUsername)
      if (result.code === 200) {
        await loadData()
        alert('已接受好友请求')
      } else {
        alert(result.message || '接受失败')
      }
    } catch (err) {
      alert('接受好友请求失败')
    }
  }

  // 拒绝好友请求
  const handleRejectFriend = async (requesterUsername) => {
    try {
      const result = await api.rejectFriendRequest(requesterUsername)
      if (result.code === 200) {
        setFriendRequests(prev => prev.filter(r => r.Username !== requesterUsername))
      }
    } catch (err) {
      console.error('拒绝失败:', err)
    }
  }

  const getConversationId = (conv) => {
    return conv.ID || conv.id
  }

  const handleSendMessage = (e) => {
    e.preventDefault()
    if (!messageInput.trim() || !selectedConversation) return
    
    const convId = getConversationId(selectedConversation)
    console.log('发送消息 - 会话ID:', convId)
    sendMessage(convId, messageInput.trim())
    setMessageInput('')
  }

  const handleFileSelect = async (e) => {
    const file = e.target.files?.[0]
    if (!file || !selectedConversation) return

    setUploading(true)
    try {
      const convId = getConversationId(selectedConversation)
      const msgType = file.type.startsWith('image/') ? 2 : 3

      // Step 1: Get presigned URL from backend
      const presignedResult = await api.getPresignedUrl(
        convId, 
        user.id, 
        msgType, 
        file.name, 
        file.size, 
        file.type
      )

      if (presignedResult.code !== 200 || !presignedResult.data?.upload_url) {
        alert(presignedResult.message || '获取上传地址失败')
        return
      }

      const { upload_url, file_url, object_key } = presignedResult.data

      // Step 2: Upload file directly to MinIO via presigned URL
      const contentType = file.type || 'application/octet-stream'
      const uploadResponse = await fetch(upload_url, {
        method: 'PUT',
        body: file,
        headers: {
          'Content-Type': contentType
        }
      })

      if (!uploadResponse.ok) {
        throw new Error('文件上传失败')
      }

      // Step 3: Confirm upload with backend (backend verifies file exists and creates message)
      const confirmResult = await api.confirmUpload(object_key, convId, user.id, msgType)
      if (confirmResult.code !== 200) {
        alert(confirmResult.message || '文件上传确认失败')
        return
      }

      // Step 4: Send message with the confirmed file URL
      const isImage = file.type.startsWith('image/')
      const content = isImage ? `[图片]${file_url}` : `[文件]${file.name}:${file_url}`
      sendMessage(convId, content)

    } catch (err) {
      console.error('上传失败:', err)
      alert('上传失败')
    } finally {
      setUploading(false)
      e.target.value = ''
    }
  }

  const handleAttachmentClick = () => {
    fileInputRef.current?.click()
  }

  const handleAvatarChange = async (e) => {
    const file = e.target.files?.[0]
    if (!file) return

    setUploadingAvatar(true)
    try {
      const result = await api.uploadUserAvatar(file)
      if (result.code === 200 && result.data) {
        const newAvatarUrl = result.data.avatar_url
        const updatedUser = { ...user, avatar_url: newAvatarUrl }
        api.setUser(updatedUser)
        window.location.reload()
      } else {
        alert(result.message || '头像上传失败')
      }
    } catch (err) {
      console.error('头像上传失败:', err)
      alert('头像上传失败')
    } finally {
      setUploadingAvatar(false)
      e.target.value = ''
    }
  }

  const handleAvatarClick = () => {
    avatarInputRef.current?.click()
  }

  const getCurrentMessages = () => {
    if (!selectedConversation) return []
    const convId = getConversationId(selectedConversation)
    const msgs = messages[convId] || []
    // 按 id 升序排序，id 越小越早
    return [...msgs].sort((a, b) => (a.id || 0) - (b.id || 0))
  }

  const getConversationName = (conv) => {
    if (conv.Name) return conv.Name
    if (conv.target_username) return conv.target_username
    return conv.type === 1 ? '私聊' : '群聊'
  }

  // 根据 sender_id 获取发送者名称
  const getSenderName = (senderId) => {
    if (senderId === user?.id) return '我'
    const friend = friends.find(f => f.ID === senderId)
    if (friend) return friend.Username
    return `用户 ${senderId}`
  }

  const handleSelectChat = (chat) => {
    // 点击会话
    setSelectedConversation(chat.conv)
  }

  const handleCreatePrivateChat = async (friendUsername) => {
    try {
      const result = await api.createPrivateConversation(friendUsername)
      if (result.code === 200 && result.data) {
        const newConv = result.data
        setConversations(prev => {
          // 检查是否已存在
          const exists = prev.find(c => c.ID === newConv.ID)
          if (exists) return prev
          return [newConv, ...prev]
        })
        setSelectedConversation(newConv)
      } else {
        alert(result.message || '创建私聊失败')
      }
    } catch (err) {
      alert('创建私聊失败')
    }
  }

  const getFileIcon = (filename) => {
    const ext = filename.split('.').pop()?.toLowerCase()
    const iconMap = {
      pdf: '📄',
      doc: '📝', docx: '📝',
      xls: '📊', xlsx: '📊',
      ppt: '📊', pptx: '📊',
      zip: '📦', rar: '📦', '7z': '📦', tar: '📦', gz: '📦',
      jpg: '🖼️', jpeg: '🖼️', png: '🖼️', gif: '🖼️', bmp: '🖼️', webp: '🖼️', svg: '🖼️',
      mp3: '🎵', wav: '🎵', flac: '🎵', aac: '🎵', ogg: '🎵',
      mp4: '🎬', avi: '🎬', mkv: '🎬', mov: '🎬', wmv: '🎬',
      txt: '📃', md: '📃', json: '📃', xml: '📃', html: '📃', css: '📃', js: '📃',
      exe: '⚙️', msi: '⚙️', dmg: '⚙️', apk: '⚙️',
      csv: '📊',
    }
    return iconMap[ext] || '📎'
  }

  const getFileName = (content) => {
    const parts = content.split(':')
    return parts.length > 1 ? parts[0].replace('[文件]', '') : content
  }

  const getFileUrl = (content) => {
    const parts = content.split(':')
    return parts.length > 1 ? parts[1] : content
  }

  const handleDownload = (url, filename) => {
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.target = '_blank'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" />
  }

  return (
    <div className="h-screen flex flex-col bg-gray-100">
      {/* 顶部导航 */}
      <header className="bg-white shadow-sm z-10">
        <div className="flex justify-between items-center px-4 py-3">
          <h1 className="text-xl font-bold text-gray-800">即时通讯</h1>
          <div className="flex items-center gap-3">
            <span className={`text-sm ${connected ? 'text-green-500' : 'text-red-500'}`}>
              {connected ? '● 在线' : '● 离线'}
            </span>
            <input
              type="file"
              ref={avatarInputRef}
              onChange={handleAvatarChange}
              className="hidden"
              accept="image/*"
            />
            <div 
              className="w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center text-white cursor-pointer overflow-hidden hover:opacity-80"
              onClick={handleAvatarClick}
              title={uploadingAvatar ? '上传中...' : '点击更换头像'}
            >
              {user?.avatar_url ? (
                <img src={user.avatar_url} alt="头像" className="w-full h-full object-cover" />
              ) : (
                <span className="text-sm font-bold">{user?.username?.charAt(0).toUpperCase()}</span>
              )}
            </div>
            <span className="text-gray-700">{user?.username}</span>
            <button
              onClick={logout}
              className="bg-red-500 text-white px-3 py-1 rounded text-sm hover:bg-red-600"
            >
              退出
            </button>
          </div>
        </div>
      </header>

      <div className="flex flex-1 overflow-hidden">
        {/* 左侧侧边栏 - 合并为单一列表 */}
        <div className="w-72 bg-white border-r flex flex-col">
          {/* 搜索和添加好友 */}
          <div className="p-3 border-b">
            <div className="flex gap-2">
              <button
                onClick={() => setShowAddFriend(true)}
                className="flex-1 bg-blue-500 text-white py-2 rounded text-sm hover:bg-blue-600"
              >
                添加好友
              </button>
              {friendRequests.length > 0 && (
                <button
                  onClick={() => setShowRequests(true)}
                  className="bg-orange-500 text-white px-3 py-2 rounded text-sm hover:bg-orange-600"
                >
                  {friendRequests.length}
                </button>
              )}
            </div>
          </div>

          {/* 会话/好友列表 */}
          <div className="flex-1 overflow-y-auto">
            {loading ? (
              <div className="p-4 text-center text-gray-500">加载中...</div>
            ) : getChatList().length === 0 ? (
              <div className="p-4 text-center text-gray-400">暂无会话</div>
            ) : (
              getChatList().map(chat => {
                const chatId = getConversationId(chat.conv || { id: chat.id })
                const selectedId = selectedConversation ? getConversationId(selectedConversation) : null
                return (
                  <div
                    key={chat.id}
                    className={`p-3 cursor-pointer hover:bg-gray-100 border-b ${selectedId === chatId ? 'bg-blue-50' : ''}`}
                    onClick={() => handleSelectChat(chat)}
                  >
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 rounded-full bg-gray-300 flex items-center justify-center text-white font-bold">
                        {chat.name.charAt(0).toUpperCase()}
                      </div>
                      <div className="flex-1">
                        <div className="font-medium">{chat.name}</div>
                        <div className="text-xs text-gray-500">
                          {chat.type === 'group' ? '群聊' : '私聊'}
                        </div>
                      </div>
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </div>

        {/* 右侧聊天区域 */}
        <div className="flex-1 flex flex-col bg-white">
          {selectedConversation ? (
            <>
              {/* 聊天头部 */}
              <div className="border-b p-3">
                <h2 className="font-bold text-lg">{getConversationName(selectedConversation)}</h2>
                <span className="text-xs text-gray-500">{(selectedConversation.type || selectedConversation.Type) === 1 ? '私聊' : '群聊'}</span>
              </div>

              {/* 消息列表 */}
              <div className="flex-1 overflow-y-auto p-4 space-y-3">
                {getCurrentMessages().map((msg, index) => {
                  const isSelf = msg.sender_id === user?.id || msg.temp_id
                  const isPending = msg.status === 'pending'
                  const senderName = getSenderName(msg.sender_id)
                  
                  return (
                    <div
                      key={msg.msg_id || msg.msg_tag || msg.temp_id || index}
                      className={`flex ${isSelf ? 'justify-end' : 'justify-start'}`}
                    >
                      <div
                        className={`max-w-xs px-4 py-2 rounded-lg ${
                          isSelf
                            ? 'bg-blue-500 text-white'
                            : 'bg-gray-200 text-gray-800'
                        }`}
                      >
                        <div className="flex items-center gap-2">
                          {!msg.content.startsWith('[图片]') && !msg.content.startsWith('[文件]') && (
                            <span>{msg.content}</span>
                          )}
                          {isPending && (
                            <span className="text-xs opacity-70">⏳</span>
                          )}
                        </div>
                        {msg.content.startsWith('[图片]') && (
                          <img 
                            src={msg.content.replace('[图片]', '')} 
                            alt="图片消息" 
                            className="mt-2 max-w-full rounded cursor-pointer"
                            onClick={() => {
                              const url = msg.content.replace('[图片]', '')
                              const filename = url.split('/').pop() || 'image'
                              handleDownload(url, filename)
                            }}
                          />
                        )}
                        {msg.content.startsWith('[文件]') && (
                          <div 
                            className="mt-2 flex items-center gap-2 cursor-pointer hover:bg-opacity-80 p-2 rounded"
                            onClick={() => {
                              const url = getFileUrl(msg.content)
                              const filename = getFileName(msg.content)
                              if (url) handleDownload(url, filename)
                            }}
                          >
                            <span className="text-lg">{getFileIcon(getFileName(msg.content))}</span>
                            <div className="flex flex-col">
                              <span className="text-sm font-medium truncate max-w-[150px]">{getFileName(msg.content)}</span>
                              <span className="text-xs opacity-70">点击下载</span>
                            </div>
                          </div>
                        )}
                        <div className="text-xs opacity-70 mt-1 flex items-center gap-1">
                          {senderName}
                          {isPending && <span className="text-yellow-300">(发送中)</span>}
                        </div>
                      </div>
                    </div>
                  )
                })}
                <div ref={messagesEndRef} />
              </div>

              {/* 消息输入框 */}
              <form onSubmit={handleSendMessage} className="border-t p-3 flex gap-2">
                <input
                  type="file"
                  ref={fileInputRef}
                  onChange={handleFileSelect}
                  className="hidden"
                />
                <button
                  type="button"
                  onClick={handleAttachmentClick}
                  disabled={uploading}
                  className="bg-gray-200 text-gray-700 px-3 py-2 rounded-lg hover:bg-gray-300 disabled:opacity-50"
                >
                  {uploading ? '上传中...' : '📎'}
                </button>
                <input
                  type="text"
                  value={messageInput}
                  onChange={(e) => setMessageInput(e.target.value)}
                  placeholder="输入消息..."
                  className="flex-1 border rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <button
                  type="submit"
                  className="bg-blue-500 text-white px-4 py-2 rounded-lg hover:bg-blue-600"
                >
                  发送
                </button>
              </form>
            </>
          ) : (
            <div className="flex-1 flex items-center justify-center text-gray-500">
              选择一个会话或好友开始聊天
            </div>
          )}
        </div>
      </div>

      {/* 添加好友弹窗 */}
      {showAddFriend && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg w-96 p-4">
            <div className="flex justify-between items-center mb-4">
              <h3 className="text-lg font-bold">添加好友</h3>
              <button onClick={() => { setShowAddFriend(false); setSearchResults([]); setSearchKeyword(''); }} className="text-gray-500">✕</button>
            </div>
            
            <div className="flex gap-2 mb-4">
              <input
                type="text"
                value={searchKeyword}
                onChange={(e) => setSearchKeyword(e.target.value)}
                onKeyPress={(e) => e.key === 'Enter' && handleSearch()}
                placeholder="输入用户名搜索"
                className="flex-1 border rounded px-3 py-2"
              />
              <button
                onClick={handleSearch}
                disabled={searching}
                className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600"
              >
                {searching ? '搜索中' : '搜索'}
              </button>
            </div>

            <div className="max-h-60 overflow-y-auto">
              {searchResults.length === 0 ? (
                <div className="text-center text-gray-500 py-4">搜索用户显示在这里</div>
              ) : (
                searchResults.map(result => (
                  <div key={result.ID} className="flex justify-between items-center p-2 border-b">
                    <span>{result.Username}</span>
                    <button
                      onClick={() => handleAddFriend(result.Username)}
                      className="text-blue-500 text-sm"
                    >
                      添加
                    </button>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}

      {/* 好友请求弹窗 */}
      {showRequests && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg w-96 p-4">
            <div className="flex justify-between items-center mb-4">
              <h3 className="text-lg font-bold">好友请求</h3>
              <button onClick={() => setShowRequests(false)} className="text-gray-500">✕</button>
            </div>
            
            <div className="max-h-60 overflow-y-auto">
              {friendRequests.length === 0 ? (
                <div className="text-center text-gray-500 py-4">暂无好友请求</div>
              ) : (
                friendRequests.map(request => (
                  <div key={request.ID} className="flex justify-between items-center p-2 border-b">
                    <span>{request.Username}</span>
                    <div className="flex gap-2">
                      <button
                        onClick={() => handleAcceptFriend(request.Username)}
                        className="text-green-500 text-sm"
                      >
                        接受
                      </button>
                      <button
                        onClick={() => handleRejectFriend(request.Username)}
                        className="text-red-500 text-sm"
                      >
                        拒绝
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}