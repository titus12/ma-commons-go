package testconsole

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"


	"github.com/titus12/ma-commons-go/utils"

	"github.com/golang/protobuf/proto"
)

// 命令
type Command struct {
	Name     string
	Callback func(message proto.Message)
}

// 控制台
type Console struct {
	//empty    func(string)
	commands []*Command
}

func NewConsole() *Console {
	cons := &Console{commands: make([]*Command, 0)}
	return cons
}

func ReadLine() (string, error) {
	text, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	text = strings.TrimRight(text, "\n\r")
	return text, nil
}

func (cons *Console) Command(name string, callback func(message proto.Message)) {
	command := &Command{
		Name:     name,
		Callback: callback,
	}
	cons.commands = append(cons.commands, command)
}

func (cons *Console) Run() {
	for {
		fmt.Print(">> ")
		text, err := ReadLine()
		if err != nil {
			panic("Error reading console input")
		}

		found := false
		for _, command := range cons.commands {
			prefix := command.Name + " "
			if strings.HasPrefix(text, prefix) {
				parts := strings.SplitN(text, " ", 2)
				if len(parts) != 2 {
					fmt.Printf("命令解析错误，LEN != 2\n")
					break
				}
				msgtype := fmt.Sprintf("testmsg.%s", command.Name)
				t := proto.MessageType(msgtype)
				if t == nil {
					fmt.Printf("不存在的消息类型, msgtype: %s\n", msgtype)
					break
				}
				msg := reflect.New(t.Elem()).Interface()

				err := json.Unmarshal(utils.StringToBytes(parts[1]), msg)
				if err != nil {
					fmt.Printf("命令解析错误, ERR: %v\n", err)
					break
				}

				found = true
				command.Callback(msg.(proto.Message))
				break
			}
		}

		if !found {
			fmt.Printf("不存在的命令\n")
		}
	}
}
