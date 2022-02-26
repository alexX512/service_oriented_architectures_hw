package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"

	"go.uber.org/zap"
)

const (
	SendVoice int = iota
	SendText
	GetUsers
	SetName
	SetRoomId
)

type Message struct {
	MsgType  int    `json:"msg_type"`
	Data     string `json:"data"`
	MetaData string `json:"meta_data"`
}

func writeData(connection *bufio.ReadWriter, message Message) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if _, err := connection.Write(data); err != nil {
		return err
	}
	if err := connection.Flush(); err != nil {
		return err
	}
	return nil
}

func writeTextData(connection *bufio.ReadWriter, message string) error {
	return writeData(connection, Message{SendText, message, ""})
}

func readData(connection *bufio.ReadWriter) (Message, error) {
	data, err := connection.ReadBytes('\n')
	if err != nil {
		return Message{}, err
	}

	message := Message{}
	if err := json.Unmarshal(data, &message); err != nil {
		return Message{}, err
	}
	return message, nil
}

func getUserRoomId(connection *bufio.ReadWriter, connMap *sync.Map) (string, error) {
	for {
		message, err := readData(connection)
		fmt.Printf("Room id from client: %s\n", message.Data)
		if err != nil {
			return "", err
		}
		infoRoom := strings.SplitN(message.Data, ":", 2)

		_, isOld := connMap.Load(infoRoom[1])
		if infoRoom[0] == "new" && isOld {
			err := writeTextData(connection, "Incorrect room id. Room id \""+infoRoom[1]+"\" isn't new")
			if err != nil {
				return "", err
			}
		} else if infoRoom[0] == "old" && !isOld {
			err := writeTextData(connection, "Incorrect room id. Room id \""+infoRoom[1]+"\" doesn't exist")
			if err != nil {
				return "", err
			}
		} else {
			err := writeTextData(connection, "Correct room id. Your room id is \""+infoRoom[1]+"\"")
			if err != nil {
				return "", err
			}
			if !isOld {
				connMap.Store(infoRoom[1], &sync.Map{})
			}
			return infoRoom[1], nil
		}
	}
}

func getUserName(connection *bufio.ReadWriter, connMap *sync.Map) (string, error) {
	for {
		message, err := readData(connection)
		fmt.Printf("User name from client: %s\n", message.Data)
		if err != nil {
			return "", err
		}
		userName := string(message.Data)

		_, loaded := connMap.LoadOrStore(userName, connection)
		if loaded {
			err := writeTextData(connection, "User name "+userName+" is occupied by another user. Write another name:")
			if err != nil {
				return "", err
			}
		} else {
			err := writeTextData(connection, "Correct name. You can start chatting")
			if err != nil {
				connMap.Delete(userName)
				return "", err
			}
			return userName, nil
		}
	}
}

func getUserMessages(userName string, connection *bufio.ReadWriter, connMap *sync.Map) (string, error) {
	for {
		message, err := readData(connection)
		fmt.Printf("Client %s send data: %s\n", userName, message.Data)
		if err != nil {
			return "Error during reading client message", err
		}

		if message.MsgType == GetUsers {
			var users []string
			connMap.Range(func(key, value interface{}) bool {
				users = append(users, key.(string))
				return true
			})

			data, err := json.Marshal(users)
			if err != nil {
				return "", err
			}

			if err := writeData(connection, Message{GetUsers, string(data), ""}); err != nil {
				return "", err
			}

		} else if message.MsgType == SendVoice {
			userVoice := message.Data
			userFile := message.MetaData

			connMap.Range(func(key, value interface{}) bool {
				if key != userName {
					if conn, ok := value.(*bufio.ReadWriter); ok {
						if err := writeData(conn, Message{SendVoice, userVoice, userName + "|" + userFile}); err != nil {
							return false
						}
					}
				}

				return true
			})
		}
	}
}

func handleUserConnection(c net.Conn, connMap *sync.Map, logger *zap.Logger) {
	defer c.Close()

	connection := bufio.NewReadWriter(bufio.NewReader(c), bufio.NewWriter(c))

	roomId, err := getUserRoomId(connection, connMap)
	if err != nil {
		logger.Error("error during getting room id from client", zap.Error(err))
		return
	}
	logger.Info("Client send correct room id: " + roomId)

	value, _ := connMap.Load(roomId)
	connMap, _ = value.(*sync.Map)
	userName, err := getUserName(connection, connMap)
	if err != nil {
		logger.Error("error during getting name from client", zap.Error(err))
		return
	}
	logger.Info("Client send correct name: " + userName)
	defer connMap.Delete(userName)

	if msg, err := getUserMessages(userName, connection, connMap); err != nil {
		logger.Error(msg, zap.Error(err))
	}
}

var SERVER_PORT = 5454

func main() {
	var loggerConfig = zap.NewProductionConfig()
	loggerConfig.Level.SetLevel(zap.DebugLevel)

	logger, err := loggerConfig.Build()
	if err != nil {
		panic(err)
	}
	logger.Info("Start server")

	l, err := net.Listen("tcp", "0.0.0.0:"+fmt.Sprint(SERVER_PORT))
	if err != nil {
		return
	}

	defer l.Close()

	var connMap = &sync.Map{}

	for {
		conn, err := l.Accept()
		if err != nil {
			logger.Error("error accepting connection", zap.Error(err))
			return
		}
		logger.Info("New connection with client")

		go handleUserConnection(conn, connMap, logger)
	}
}
