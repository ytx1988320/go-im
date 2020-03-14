/**
 * Created by GoLand.
 * User: link1st
 * Date: 2019-07-25
 * Time: 16:24
 */

package websocket

import (
	"github.com/gorilla/websocket"
	"go-im/logger"
	"go.uber.org/zap"
	"runtime/debug"
)

const (
	// 用户连接超时时间
	heartbeatExpirationTime = 6 * 60
)

// 用户登录
type login struct {
	AppId  uint32
	UserId string
	Client *Client
}

// 读取客户端数据
func (l *login) GetKey() (key string) {
	key = GetUserKey(l.AppId, l.UserId)

	return
}

// 用户连接
type Client struct {
	Addr          string          // 客户端地址
	Socket        *websocket.Conn // 用户连接
	Send          chan []byte     // 待发送的数据
	AppId         uint32          // 登录的平台Id app/web/ios
	UserId        string          // 用户Id，用户登录以后才有
	FirstTime     uint64          // 首次连接事件
	HeartbeatTime uint64          // 用户上次心跳时间
	LoginTime     uint64          // 登录时间 登录以后才有
}

// 初始化
func NewClient(addr string, socket *websocket.Conn, firstTime uint64) (client *Client) {
	client = &Client{
		Addr:          addr,
		Socket:        socket,
		Send:          make(chan []byte, 100),
		FirstTime:     firstTime,
		HeartbeatTime: firstTime,
	}

	return
}

// 读取客户端数据
func (c *Client) GetKey() (key string) {
	key = GetUserKey(c.AppId, c.UserId)

	return
}

// 读取客户端数据
func (c *Client) read() {
	defer func() {
		if r := recover(); r != nil {
			logger.Logger.Error("write stop", zap.Any("r", r), zap.Binary("Stack", debug.Stack()))
		}
	}()

	defer func() {
		logger.Logger.Info("读取客户端数据 关闭send", zap.Any("c", c))
		close(c.Send)
	}()

	for {
		_, message, err := c.Socket.ReadMessage()
		if err != nil {
			logger.Logger.Error("读取客户端数据 错误", zap.String("Addr", c.Addr), zap.Any("err", err))
			return
		}
		// 处理程序
		logger.Logger.Info("读取客户端数据 处理:", zap.Binary("message", message))
		ProcessData(c, message)
	}
}

// 向客户端写数据
func (c *Client) write() {
	defer func() {
		if r := recover(); r != nil {
			logger.Logger.Error("write stop:", zap.Any("r", r), zap.Binary("Stack", debug.Stack()))
		}
	}()
	defer func() {
		clientManager.Unregister <- c
		c.Socket.Close()
		logger.Logger.Info("Client发送数据 defer", zap.Any("c", c))
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				// 发送数据错误 关闭连接
				logger.Logger.Info("Client发送数据 关闭连接", zap.String("Addr", c.Addr), zap.Bool("ok", ok))
				return
			}
			c.Socket.WriteMessage(websocket.TextMessage, message)
		}
	}
}

// 读取客户端数据
func (c *Client) SendMsg(msg []byte) {

	if c == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Logger.Error("SendMsg stop:", zap.Any("r", r), zap.Binary("stack", debug.Stack()))
		}
	}()

	c.Send <- msg
}

// 读取客户端数据
func (c *Client) close() {
	close(c.Send)
}

// 用户登录
func (c *Client) Login(appId uint32, userId string, loginTime uint64) {
	c.AppId = appId
	c.UserId = userId
	c.LoginTime = loginTime
	// 登录成功=心跳一次
	c.Heartbeat(loginTime)
}

// 用户心跳
func (c *Client) Heartbeat(currentTime uint64) {
	c.HeartbeatTime = currentTime
	return
}

// 心跳超时
func (c *Client) IsHeartbeatTimeout(currentTime uint64) (timeout bool) {
	if c.HeartbeatTime+heartbeatExpirationTime <= currentTime {
		timeout = true
	}
	return
}

// 是否登录了
func (c *Client) IsLogin() (isLogin bool) {

	// 用户登录了
	if c.UserId != "" {
		isLogin = true

		return
	}

	return
}
