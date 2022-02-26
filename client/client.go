package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

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

func writeData(connection *bufio.Writer, message Message) error {
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

func readData(connection *bufio.Reader) (Message, error) {
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

func printValidCommands() {
	fmt.Println("> You can use next commands for communication with app:")
	fmt.Println("> \\SendVoice FileName - you send to all clients file with name \"FileName\"")
	fmt.Println("> \\GetUsers - you get names of all active users in chat")
	fmt.Println("> \\Stop - you stop app")
}

func setRoomId(reader *bufio.Reader, writer *bufio.Writer) error {
	var answer string
	fmt.Println("> Do you want to create new room [Yes/No]:")
	fmt.Scanf("%s\n", &answer)

	var info string
	if answer == "Yes" {
		fmt.Println("> Enter your new room id:")
		info = "new"
	} else {
		fmt.Println("> Enter your room id:")
		info = "old"
	}

	for {
		var err error
		var roomId string
		fmt.Scanf("%s\n", &roomId)

		if err = writeData(writer, Message{SetRoomId, info + ":" + roomId, ""}); err != nil {
			return err
		}

		var isCorrect Message
		if isCorrect, err = readData(reader); err != nil {
			return err
		}
		fmt.Println("> " + isCorrect.Data)
		if strings.Split(isCorrect.Data, ".")[0] == "Correct room id" {
			break
		}
	}

	return nil
}

func setName(reader *bufio.Reader, writer *bufio.Writer) error {
	fmt.Println("> Enter your name:")
	for {
		var err error
		var name string
		if _, err = fmt.Scanf("%s\n", &name); err != nil {
			return err
		}

		if err = writeData(writer, Message{SetName, name, ""}); err != nil {
			return err
		}

		var isCorrect Message
		if isCorrect, err = readData(reader); err != nil {
			return err
		}
		fmt.Println("> " + isCorrect.Data)

		if isCorrect.Data == "Correct name. You can start chatting" {
			break
		}
	}

	printValidCommands()
	return nil
}

func sendClientMessages(writer *bufio.Writer, ch chan error) {
	for {
		var msgType int
		var data string = ""
		var metaData string = ""

		clientMessage, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		clientMessage = strings.TrimSuffix(clientMessage, "\r\n")
		clientMessage = strings.TrimSuffix(clientMessage, "\n")

		if strings.HasPrefix(clientMessage, "\\SendVoice ") {
			info := strings.SplitAfter(clientMessage, " ")

			byteData, err := ioutil.ReadFile(info[1])
			if err != nil {
				ch <- err
				return
			}

			msgType = SendVoice
			data = string(byteData)
			metaData = info[1]

		} else if clientMessage == "\\GetUsers" {
			msgType = GetUsers

		} else if clientMessage == "\\Stop" {
			ch <- nil
			return

		} else {
			fmt.Println("Wrong type of command")
			continue
		}

		if err := writeData(writer, Message{msgType, data, metaData}); err != nil {
			ch <- err
			return
		}
	}
}

func getServerMessages(reader *bufio.Reader, ch chan error) {
	for {
		message, err := readData(reader)
		if err != nil {
			ch <- err
			return
		}

		if message.MsgType == GetUsers {
			var users []string
			err = json.Unmarshal([]byte(message.Data), &users)
			if err != nil {
				ch <- err
				return
			}

			fmt.Println("> Users:")
			for index, user := range users {
				fmt.Printf("> %d) %s\n", index, user)
			}

		} else if message.MsgType == SendText {
			fmt.Println("> " + message.Data)

		} else if message.MsgType == SendVoice {
			info := strings.Split(message.MetaData, "|")
			sender, fileName := info[0], info[1]

			fmt.Printf("New message from %s in file \"%s\"\n", sender, fileName)
			err = ioutil.WriteFile(fileName, []byte(message.Data), 0644)
			if err != nil {
				ch <- err
				return
			}
		}

	}
}

func communicate(reader *bufio.Reader, writer *bufio.Writer) error {
	ch := make(chan error)
	go getServerMessages(reader, ch)
	go sendClientMessages(writer, ch)

	err := <-ch
	return err
}

func handleConnection(c net.Conn, logger *zap.Logger) {
	writer := bufio.NewWriter(c)
	reader := bufio.NewReader(c)

	if err := setRoomId(reader, writer); err != nil {
		logger.Error("error during getting room id from client", zap.Error(err))
		return
	}
	logger.Info("Room id was chosen")

	if err := setName(reader, writer); err != nil {
		logger.Error("error during getting name from client", zap.Error(err))
		return
	}
	logger.Info("Name was setted")

	if err := communicate(reader, writer); err != nil {
		logger.Error("error during communication", zap.Error(err))
		return
	}
}

func startConnection() net.Conn {
	for {
		var serverIp, port string
		fmt.Println("> Enter server ip:")
		fmt.Scanln(&serverIp)

		fmt.Println("> Enter server port:")
		fmt.Scanln(&port)

		serverAddress := serverIp + ":" + port
		c, err := net.Dial("tcp", serverAddress)
		if err != nil {
			fmt.Printf("> Problems with connection to server \"%s\". Error: %s\n", serverAddress, err)
		} else {
			return c
		}
	}
}

func main() {
	var loggerConfig = zap.NewProductionConfig()
	loggerConfig.Level.SetLevel(zap.ErrorLevel)

	logger, err := loggerConfig.Build()
	if err != nil {
		panic(err)
	}
	logger.Info("Start client")

	c := startConnection()
	defer c.Close()
	logger.Info("Start connection with server")

	handleConnection(c, logger)
}
