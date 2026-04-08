export const API_BASE = '/api/v1'

const api = {
  baseURL: API_BASE,
  
  getToken() {
    return localStorage.getItem('token')
  },
  
  getRefreshToken() {
    return localStorage.getItem('refreshToken')
  },
  
  setTokens(accessToken, refreshToken) {
    localStorage.setItem('token', accessToken)
    localStorage.setItem('refreshToken', refreshToken)
  },
  
  clearTokens() {
    localStorage.removeItem('token')
    localStorage.removeItem('refreshToken')
    localStorage.removeItem('user')
  },
  
  setUser(user) {
    localStorage.setItem('user', JSON.stringify(user))
  },
  
  getUser() {
    const userStr = localStorage.getItem('user')
    return userStr ? JSON.parse(userStr) : null
  },

  async request(method, url, data = null, headers = {}) {
    const token = this.getToken()
    const config = {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        ...headers,
      },
    }
    
    if (data && method !== 'GET') {
      config.body = JSON.stringify(data)
    }
    
    const response = await fetch(`${this.baseURL}${url}`, config)
    const result = await response.json()
    
    if (!response.ok) {
      throw new Error(result.message || '请求失败')
    }
    
    return result
  },

  get(url) {
    return this.request('GET', url)
  },

  post(url, data) {
    return this.request('POST', url, data)
  },

  async uploadFile(url, formData) {
    const token = this.getToken()
    const response = await fetch(`${this.baseURL}${url}`, {
      method: 'POST',
      headers: {
        ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
      },
      body: formData,
    })
    const result = await response.json()
    if (!response.ok) {
      throw new Error(result.message || '上传失败')
    }
    return result
  },

  // ============ 会话相关 API ============
  async getConversations() {
    return this.post('/conversations/listConversations', {})
  },

  async createPrivateConversation(target) {
    return this.post('/conversations/createPrivate', { target })
  },

  async createGroupConversation(target, groupName) {
    return this.post('/conversations/createGroupe', { target, group_name: groupName })
  },

  async getConversationMembers(conversationId) {
    return this.post('/conversations/members', { conversation_id: conversationId })
  },

  // ============ 好友相关 API ============
  async getFriendList() {
    return this.post('/friends/list', { page: 1, page_size: 50 })
  },

  async getFriendRequests() {
    return this.get('/friends/requests')
  },

  async addFriend(targetUsername) {
    return this.post('/friends/add', { target_username: targetUsername })
  },

  async acceptFriendRequest(requesterUsername) {
    return this.post('/friends/accept', { requester_username: requesterUsername })
  },

  async rejectFriendRequest(requesterUsername) {
    return this.post('/friends/reject', { requester_username: requesterUsername })
  },

  async deleteFriend(targetUsername) {
    return this.post('/friends/delete', { target_username: targetUsername })
  },

  async searchUsers(keyword) {
    return this.post('/users/search', { keyword, page: 1, page_size: 20 })
  },

  // ============ 群组相关 API ============
  async inviteGroupMember(conversationId, targetUsername) {
    return this.post('/groups/invite', { conversation_id: conversationId, target_username: targetUsername })
  },

  async kickGroupMember(conversationId, targetUsername) {
    return this.post('/groups/kick', { conversation_id: conversationId, target_username: targetUsername })
  },

  async leaveGroup(conversationId) {
    return this.post('/groups/leave', { conversation_id: conversationId })
  },

  async updateGroupInfo(conversationId, groupName, avatarUrl) {
    return this.post('/groups/update-info', { conversation_id: conversationId, group_name: groupName, avatar_url: avatarUrl })
  },

  async transferGroupOwner(conversationId, newOwnerUsername) {
    return this.post('/groups/transfer', { conversation_id: conversationId, new_owner_username: newOwnerUsername })
  },

  async getGroupAnnouncement(conversationId) {
    return this.post('/groups/get-announcement', { conversation_id: conversationId })
  },

  async updateGroupAnnouncement(conversationId, announcement) {
    return this.post('/groups/announcement', { conversation_id: conversationId, announcement })
  },

  // ============ 用户资料 API ============
  async getUserDetail() {
    return this.get('/users/detail')
  },

  async updateUserProfile(birthday, bio, gender) {
    return this.post('/users/profile', { birthday, bio, gender })
  },

  async updateUserAvatar(avatarUrl) {
    return this.post('/users/avatar', { avatar_url: avatarUrl })
  },

  async uploadUserAvatar(file) {
    const formData = new FormData()
    formData.append('file', file)
    return this.uploadFile('/users/uploadAvatar', formData)
  },

  async uploadGroupAvatar(file, conversationId) {
    const formData = new FormData()
    formData.append('file', file)
    return this.uploadFile('/groups/uploadAvatar', formData)
  },

  async uploadMessageFile(file, conversationId, senderId, msgType = 2) {
    const formData = new FormData()
    formData.append('data', JSON.stringify({
      conversation_id: conversationId,
      sender_id: senderId,
      msg_type: msgType
    }))
    formData.append('file', file)
    return this.uploadFile('/messages/uploadFile', formData)
  },

  async getPresignedUrl(conversationId, senderId, msgType, fileName, fileSize, contentType) {
    return this.post('/messages/presignedUrl', {
      conversation_id: conversationId,
      sender_id: senderId,
      msg_type: msgType,
      file_name: fileName,
      file_size: fileSize,
      content_type: contentType
    })
  },

  async confirmUpload(objectKey, conversationId, senderId, msgType) {
    return this.post('/messages/confirmUpload', {
      object_key: objectKey,
      conversation_id: conversationId,
      sender_id: senderId,
      msg_type: msgType
    })
  },
}

export default api