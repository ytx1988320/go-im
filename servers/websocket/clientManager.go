/**
 * Created by GoLand.
 * User: link1st
 * Date: 2019-07-25
 * Time: 16:24
 */

package websocket

import (
	"fmt"
	"go-im/cache"
	"go-im/logger"
	"go-im/model"
	"go-im/util"
	"go.uber.org/zap"
	"strings"
	"sync"
	"time"
)

// 连接管理
type ClientManager struct {
	Clients     map[*Client]bool   // 全部的连接
	ClientsLock sync.RWMutex       // 读写锁
	Users       map[string]*Client // 登录的用户 // appId+uuid
	UserLock    sync.RWMutex       // 读写锁
	Register    chan *Client       // 连接连接处理
	Login       chan *login        // 用户登录处理
	Unregister  chan *Client       // 断开连接处理程序
	Broadcast   chan []byte        // 广播 向全部成员发送数据
}

func NewClientManager() (clientManager *ClientManager) {
	clientManager = &ClientManager{
		Clients:    make(map[*Client]bool),
		Users:      make(map[string]*Client),
		Register:   make(chan *Client, 1000),
		Login:      make(chan *login, 1000),
		Unregister: make(chan *Client, 1000),
		Broadcast:  make(chan []byte, 1000),
	}

	return
}

// 获取用户key
func GetUserKey(appId uint32, userId string) (key string) {
	key = fmt.Sprintf("%d_%s", appId, userId)
	return
}

/**************************  manager  ***************************************/

func (manager *ClientManager) InClient(client *Client) (ok bool) {
	manager.ClientsLock.RLock()
	defer manager.ClientsLock.RUnlock()
	// 连接存在，在添加
	_, ok = manager.Clients[client]

	return
}

// GetClients
func (manager *ClientManager) GetClients() (clients map[*Client]bool) {

	clients = make(map[*Client]bool)

	manager.ClientsRange(func(client *Client, value bool) (result bool) {
		clients[client] = value

		return true
	})

	return
}

// 遍历
func (manager *ClientManager) ClientsRange(f func(client *Client, value bool) (result bool)) {

	manager.ClientsLock.RLock()
	defer manager.ClientsLock.RUnlock()

	for key, value := range manager.Clients {
		result := f(key, value)
		if result == false {
			return
		}
	}

	return
}

// GetClientsLen
func (manager *ClientManager) GetClientsLen() (clientsLen int) {

	clientsLen = len(manager.Clients)

	return
}

// 添加客户端
func (manager *ClientManager) AddClients(client *Client) {
	manager.ClientsLock.Lock()
	defer manager.ClientsLock.Unlock()

	manager.Clients[client] = true
}

// 删除客户端
func (manager *ClientManager) DelClients(client *Client) {
	manager.ClientsLock.Lock()
	defer manager.ClientsLock.Unlock()

	if _, ok := manager.Clients[client]; ok {
		delete(manager.Clients, client)
	}
}

// 获取用户的连接
func (manager *ClientManager) GetUserClient(appId uint32, userId string) (client *Client) {

	manager.UserLock.RLock()
	defer manager.UserLock.RUnlock()

	userKey := GetUserKey(appId, userId)
	if value, ok := manager.Users[userKey]; ok {
		client = value
	}
	return
}

// GetClientsLen
func (manager *ClientManager) GetUsersLen() (userLen int) {
	userLen = len(manager.Users)

	return
}

// 添加用户
func (manager *ClientManager) AddUsers(key string, client *Client) {
	manager.UserLock.Lock()
	defer manager.UserLock.Unlock()

	manager.Users[key] = client
}

// 删除用户
func (manager *ClientManager) DelUsers(client *Client) (result bool) {
	manager.UserLock.Lock()
	defer manager.UserLock.Unlock()

	key := GetUserKey(client.AppId, client.UserId)
	if value, ok := manager.Users[key]; ok {
		// 判断是否为相同的用户
		if value.Addr != client.Addr {

			return
		}
		delete(manager.Users, key)
		result = true
	}

	return
}

// 获取用户的key
func (manager *ClientManager) GetUserKeys() (userKeys []string) {

	userKeys = make([]string, 0)
	manager.UserLock.RLock()
	defer manager.UserLock.RUnlock()
	for key := range manager.Users {
		userKeys = append(userKeys, key)
	}

	return
}

// 获取用户的key
func (manager *ClientManager) GetUserList() (userList []string) {

	userList = make([]string, 0)

	clientManager.UserLock.RLock()
	defer clientManager.UserLock.RUnlock()

	for _, v := range clientManager.Users {
		userList = append(userList, v.UserId)
		logger.Logger.Debug("GetUserList", zap.Uint32("AppId", v.AppId),
			zap.String("UserId", v.UserId),
			zap.String("Addr", v.Addr))
	}

	logger.Logger.Info("GetUserList", zap.Any("Users", clientManager.Users))
	return
}

// 获取用户的链接
func (manager *ClientManager) GetUserClients() (clients []*Client) {
	clients = make([]*Client, 0)
	manager.UserLock.RLock()
	defer manager.UserLock.RUnlock()
	for _, v := range manager.Users {
		clients = append(clients, v)
	}
	return
}

//获取房间类的用户链接
func (manager *ClientManager) GetRoomUserClients(appId uint32) (clients []*Client) {
	clients = make([]*Client, 0)
	manager.UserLock.RLock()
	defer manager.UserLock.RUnlock()
	roomId := fmt.Sprintf("%d", appId)
	for key, v := range manager.Users {
		if strings.Contains(key, roomId) {
			clients = append(clients, v)
		}
	}
	return
}

// 向全部成员(除了自己)发送数据
func (manager *ClientManager) sendAll(message []byte, ignore *Client) {
	clients := manager.GetUserClients()
	for _, conn := range clients {
		if conn != ignore {
			conn.SendMsg(message)
		}
	}
}

/**
向房间类不是自己的人发消息
*/
func (manager *ClientManager) sendRoomAll(message []byte, appId uint32, ignore *Client) {
	clients := manager.GetRoomUserClients(appId)
	for _, conn := range clients {
		if conn != ignore {
			conn.SendMsg(message)
		}
	}
}

// 用户建立连接事件
func (manager *ClientManager) EventRegister(client *Client) {
	manager.AddClients(client)
	logger.Logger.Info("EventRegister 用户建立连接", zap.String("Addr", client.Addr))
}

// 用户登录
func (manager *ClientManager) EventLogin(login *login) {
	client := login.Client
	// 连接存在，在添加
	if manager.InClient(client) {
		userKey := login.GetKey()
		manager.AddUsers(userKey, login.Client)
	}

	logger.Logger.Info("EventLogin 用户登录",
		zap.String("Addr", client.Addr),
		zap.Uint32("AppId", login.AppId),
		zap.String("UserId", login.UserId))
	orderId := util.GetOrderIdTime()
	SendUserMessageAll(login.AppId, login.UserId, orderId, model.MessageCmdEnter, "哈喽~")
}

// 用户断开连接
func (manager *ClientManager) EventUnregister(client *Client) {
	manager.DelClients(client)

	// 删除用户连接
	deleteResult := manager.DelUsers(client)
	if deleteResult == false {
		// 不是当前连接的客户端

		return
	}

	// 清除redis登录数据
	userOnline, err := cache.GetUserOnlineInfo(client.GetKey())
	if err == nil {
		userOnline.LogOut()
		cache.SetUserOnlineInfo(client.GetKey(), userOnline)
	}
	// 关闭 chan
	// close(client.Send)
	logger.Logger.Info("EventUnregister 用户断开连接",
		zap.String("Addr", client.Addr),
		zap.Uint32("AppId", client.AppId),
		zap.String("UserId", client.UserId))

	if client.UserId != "" {
		orderId := util.GetOrderIdTime()
		SendUserMessageAll(client.AppId, client.UserId, orderId, model.MessageCmdExit, "用户已经离开~")
	}
}

// 管道处理程序
func (manager *ClientManager) start() {
	for {
		select {
		case conn := <-manager.Register:
			// 建立连接事件
			manager.EventRegister(conn)

		case login := <-manager.Login:
			// 用户登录
			manager.EventLogin(login)

		case conn := <-manager.Unregister:
			// 断开连接事件
			manager.EventUnregister(conn)

		case message := <-manager.Broadcast:
			// 广播事件
			clients := manager.GetClients()
			for conn := range clients {
				select {
				case conn.Send <- message:
				default:
					close(conn.Send)
				}
			}
		}
	}
}

/**************************  manager info  ***************************************/
// 获取管理者信息
func GetManagerInfo(isDebug string) (managerInfo map[string]interface{}) {
	managerInfo = make(map[string]interface{})

	managerInfo["clientsLen"] = clientManager.GetClientsLen()
	managerInfo["usersLen"] = clientManager.GetUsersLen()
	managerInfo["chanRegisterLen"] = len(clientManager.Register)
	managerInfo["chanLoginLen"] = len(clientManager.Login)
	managerInfo["chanUnregisterLen"] = len(clientManager.Unregister)
	managerInfo["chanBroadcastLen"] = len(clientManager.Broadcast)

	if isDebug == "true" {
		addrList := make([]string, 0)
		clientManager.ClientsRange(func(client *Client, value bool) (result bool) {
			addrList = append(addrList, client.Addr)

			return true
		})

		users := clientManager.GetUserKeys()

		managerInfo["clients"] = addrList
		managerInfo["users"] = users
	}

	return
}

// 获取用户所在的连接
func GetUserClient(appId uint32, userId string) (client *Client) {
	client = clientManager.GetUserClient(appId, userId)
	return
}

// 定时清理超时连接
func ClearTimeoutConnections() {
	currentTime := uint64(time.Now().Unix())

	clients := clientManager.GetClients()
	for client := range clients {
		if client.IsHeartbeatTimeout(currentTime) {
			logger.Logger.Info("心跳时间超时 关闭连接",
				zap.String("Addr", client.Addr), zap.String("UserId", client.UserId),
				zap.Uint64("LoginTime", client.LoginTime),
				zap.Uint64("HeartbeatTime", client.HeartbeatTime))
			client.Socket.Close()
			//删除链接
			clientManager.Unregister <- client
		}
	}
}

// 获取全部用户
func GetUserList() (userList []string) {
	userList = clientManager.GetUserList()
	return
}

// 全员广播
func AllSendMessages(appId uint32, userId string, data string) {
	logger.Logger.Info("全员广播", zap.Uint32("appId", appId), zap.String("userId", userId), zap.String("data", data))
	ignore := clientManager.GetUserClient(appId, userId)
	clientManager.sendAll([]byte(data), ignore)
}

//根据 appId房间号发送给房间人员
func AllSendRoomMessages(appId uint32, userId string, data string) {
	logger.Logger.Info("全员广播", zap.Uint32("appId", appId), zap.String("userId", userId), zap.String("data", data))
	ignore := clientManager.GetUserClient(appId, userId)
	clientManager.sendRoomAll([]byte(data), appId, ignore)
}
