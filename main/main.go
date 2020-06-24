package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
	"log"
	"net/http"
)

const (
	LoginType   = "login"
	UpdateType  = "update"
	MessageType = "message"
)

type ClientManager struct {
	clients    map[*ClientRead]bool
	broadcast  chan []byte
	register   chan *ClientRead
	unregister chan *ClientRead
}

type ClientRead struct {
	id     string
	socket *websocket.Conn
	send   chan []byte
}

type Message struct {
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Content   string `json:"content,omitempty"`
}

var manager = ClientManager{
	broadcast:  make(chan []byte),
	register:   make(chan *ClientRead),
	unregister: make(chan *ClientRead),
	clients:    make(map[*ClientRead]bool),
}

type Welcome struct {
	Type string `json:"type"`
	Id   string `json:"id"`
}

type ReceiveContent struct {
	Type     string `json:"type"`
	X        string `json:"x"`
	Y        string `json:"y"`
	Angle    string `json:"angle"`
	Momentum string `json:"momentum"`
}

type ReceiveMessage struct {
	Sender  string `json:"sender"`
	Content string `json:"content"`
}

/**
 * 要发送给其他客户端的内容
 */
type SendChatInfo struct {
	Type    string `json:"type"`
	Id      string `json:"id"`
	Message string `json:"message"`
}

/**
 * 收到別的玩家位置信息   {"type":"update","id":398331,"angle":-4.07,"momentum":0.002,"x":-24.6,"y":-138.7,"name":"dizzys","sex":1,"icon":"\/icon\/radevoftq09grjbjr1cepgg085\/thumbnail\/-%2026.jpg"}
 */
type LocationInfo struct {
	Type     string `json:"type"`
	Id       string `json:"id"`
	Momentum string `json:"momentum"`
	Angle    string `json:"angle"`
	X        string `json:"x"`
	Y        string `json:"y"`
	Name     string `json:"name"`
	Sex      string `json:"sex"`
	Icon     string `json:"icon"`
}

/**
 * 离线发送  {"type":"closed","id":398156}
 */
type SendOffline struct {
	Type string `json:"type"`
	Id   string `json:"id"`
}

func (manager *ClientManager) start() {
	for {
		select {
		case conn := <-manager.register:
			log.Printf("客户端：%s 已经加入连接", conn.id)
			// 打招呼
			welcome := Welcome{
				"welcome",
				conn.id,
			}
			jsonMessage, _ := json.Marshal(welcome)
			log.Printf("新的客户端连接分配ID ：%s\n", string(jsonMessage))
			manager.clients[conn] = true
			// 第一次发送坐标
			content := LocationInfo{
				"update",
				conn.id,
				"0.036",
				"3.063",
				"0",
				"0",
				conn.id,
				"-1",
				"",
			}
			jsonInfo, _ := json.Marshal(content)
			conn.send <- jsonMessage
			conn.send <- jsonInfo
			//
			//manager.send(jsonMessage, conn)
			//manager.send(jsonInfo, conn)
		case conn := <-manager.unregister:
			if _, ok := manager.clients[conn]; ok {
				log.Printf("客户端：%s 已经断开连接", conn.id)
				close(conn.send)
				delete(manager.clients, conn)
				offline := SendOffline{
					"closed",
					conn.id,
				}
				marshal, _ := json.Marshal(offline)
				manager.send(marshal, conn)
			}
		case message := <-manager.broadcast:
			log.Printf("收到消息：%s", string(message))
			receiveMessage := ReceiveMessage{}
			receiveErr := json.Unmarshal(message, &receiveMessage)
			if nil != receiveErr {
				log.Println(receiveErr)
				break
			}
			// 然后判断是那个type“
			// 消息分多种， 分别是移动，消息 ，还有登录
			contentTeml := LocationInfo{}
			err := json.Unmarshal([]byte(receiveMessage.Content), &contentTeml)
			if err != nil {
				log.Println(err)
				break
			}
			contentType := contentTeml.Type
			switch contentType {
			case LoginType:
				// 登录
				log.Printf("ID ：%s 登录成功.", receiveMessage.Sender)
				break
			case MessageType:
				// 发送消息 // 向大家说
				chatInfo := SendChatInfo{}
				_ = json.Unmarshal([]byte(receiveMessage.Content), &chatInfo)
				chatInfo.Id = receiveMessage.Sender
				jsonInfo, _ := json.Marshal(chatInfo)
				log.Printf("广播：%s", string(jsonInfo))
				for conn := range manager.clients {
					conn.send <- jsonInfo
				}
			case UpdateType:
				content := LocationInfo{
					"update",
					receiveMessage.Sender,
					contentTeml.Momentum,
					contentTeml.Angle,
					contentTeml.X,
					contentTeml.Y,
					contentTeml.Name,
					contentTeml.Sex,
					contentTeml.Icon,
				}
				jsonLocation, _ := json.Marshal(content)
				for conn := range manager.clients {
					content.Id = conn.id
					conn.send <- jsonLocation
				}
			default:
			}
		}
	}
}

func (manager *ClientManager) send(message []byte, ignore *ClientRead) {
	for conn := range manager.clients {
		if conn != ignore {
			conn.send <- message
		}
	}
}

func (c *ClientRead) read() {
	defer func() {
		manager.unregister <- c
		c.socket.Close()
	}()

	for {
		_, message, err := c.socket.ReadMessage()
		if err != nil {
			manager.unregister <- c
			c.socket.Close()
			break
		}
		jsonMessage, _ := json.Marshal(&Message{Sender: c.id, Content: string(message)})
		manager.broadcast <- jsonMessage
	}
}

func (c *ClientRead) write() {
	defer func() {
		c.socket.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.socket.WriteMessage(websocket.TextMessage, message)
		}
	}
}

func main() {
	fmt.Println("Starting application...")
	go manager.start()
	http.HandleFunc("/ws", wsPage)
	http.ListenAndServe(":2508", nil)
}

func wsPage(res http.ResponseWriter, req *http.Request) {
	conn, error := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(res, req, nil)
	if error != nil {
		http.NotFound(res, req)
		return
	}
	client := &ClientRead{id: uuid.NewV4().String(), socket: conn, send: make(chan []byte)}
	manager.register <- client

	go client.read()
	go client.write()
}
