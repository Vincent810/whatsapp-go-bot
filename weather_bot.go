package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"strings"
	"time"
	qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/Rhymen/go-whatsapp"
	"github.com/Rhymen/go-whatsapp/binary/proto"
	owm "github.com/briandowns/openweathermap"
	gt "github.com/bas24/googletranslatefree"
)

type waHandler struct {
	wac       *whatsapp.Conn
	startTime uint64
}

func process_msg(input string) string {
	input = strings.ReplaceAll(input, "@gobot", "")
	words := strings.Split(input, ",")
	if len(words) < 2 {
		return "need more fields separated by commas, the first field can be weather or xxx"
	}
	commd := strings.ToLower(strings.ReplaceAll(words[0], " ", ""))
	text := strings.TrimSpace(words[1])
	if len(text) < 1 {
		return "Need to input the more information separated by commas"
	}
	var output string
	switch commd {
	case "weather":
        w, err := owm.NewCurrent("C", "EN", "ba97d20de7e36445e24392a33a4c693a")
        if err != nil {
            log.Fatalln(err)
		}
		w.CurrentByName(text)
		if len(w.Weather) < 1 {
			return "The location could not be found."
		}
        output = fmt.Sprintf("Location (%s): %s, Temp(high): %.2f, Temp(low): %.2f, Temp(current): %.2f, humidity: %d, conditions: %s, wind speed (deg): %.2f (%.2f)", 
											w.Sys.Country, text, w.Main.TempMax, w.Main.TempMin, w.Main.Temp, 
											w.Main.Humidity, w.Weather[0].Description, w.Wind.Speed, w.Wind.Deg)
	case "trans-en":
		output, _ = gt.Translate(text, "en", "zh-CN")
	case "trans-zh":
		output, _ = gt.Translate(text, "zh-CN", "en")
	default:
		output = fmt.Sprintf("Command %s is not supported.", commd)
	}
	return output
}

//HandleError needs to be implemented to be a valid WhatsApp handler
func (wh *waHandler) HandleError(err error) {
	fmt.Fprintf(os.Stderr, "error caught in handler: %v\n", err)
}


//Optional to be implemented. Implement HandleXXXMessage for the types you need.
func (wh *waHandler) HandleTextMessage(message whatsapp.TextMessage) {
	if !strings.Contains(strings.ToLower(message.Text), "@gobot") || message.Info.Timestamp < wh.startTime {
		return
	}
	var output string
	output = process_msg(message.Text)

	previousMessage := "hello"
	quotedMessage := proto.Message{
		Conversation: &previousMessage,
	}

	ContextInfo := whatsapp.ContextInfo{
		QuotedMessage:   &quotedMessage,
		QuotedMessageID: "",
		Participant:     "", //Who sent the original message
	}

	msg := whatsapp.TextMessage{
		Info: whatsapp.MessageInfo{
			RemoteJid: message.Info.RemoteJid,
			},
		ContextInfo: ContextInfo,
		Text: output,
	}

	msgId, err := wh.wac.Send(msg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error sending message: %v", err)
		os.Exit(1)
	} else {
		fmt.Println("Message Sent -> ID : " + msgId)
	}
	fmt.Printf("%v %v: \t resv: %v \t resp: %v\n", message.Info.Timestamp, message.Info.RemoteJid, message.Text, output)
}

func main() {
	//create new WhatsApp connection
	wac, err := whatsapp.NewConn(5 * time.Second)
	if err != nil {
		log.Fatalf("error creating connection: %v\n", err)
	}

	//Add handler
	wac.AddHandler(&waHandler{wac, uint64(time.Now().Unix())})

	//login or restore
	if err := login(wac); err != nil {
		log.Fatalf("error logging in: %v\n", err)
	}

	//verifies phone connectivity
	pong, err := wac.AdminTest()

	if !pong || err != nil {
		log.Fatalf("error pinging in: %v\n", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	//Disconnect safe
	fmt.Println("Shutting down now.")
	session, err := wac.Disconnect()
	if err != nil {
		log.Fatalf("error disconnecting: %v\n", err)
	}
	if err := writeSession(session); err != nil {
		log.Fatalf("error saving session: %v", err)
	}
}

func login(wac *whatsapp.Conn) error {
	//load saved session
	session, err := readSession()
	if err == nil {
		//restore session
		session, err = wac.RestoreWithSession(session)
		if err != nil {
			return fmt.Errorf("restoring failed: %v\n", err)
		}
	} else {
		//no saved session -> regular login
		qr := make(chan string)
		go func() {
			terminal := qrcodeTerminal.New()
			terminal.Get(<-qr).Print()
		}()
		session, err = wac.Login(qr)
		if err != nil {
			return fmt.Errorf("error during login: %v\n", err)
		}
	}

	//save session
	err = writeSession(session)
	if err != nil {
		return fmt.Errorf("error saving session: %v\n", err)
	}
	return nil
}

func readSession() (whatsapp.Session, error) {
	session := whatsapp.Session{}
	file, err := os.Open(os.TempDir() + "/whatsappSession.gob")
	if err != nil {
		return session, err
	}
	defer file.Close()
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&session)
	if err != nil {
		return session, err
	}
	return session, nil
}

func writeSession(session whatsapp.Session) error {
	file, err := os.Create(os.TempDir() + "/whatsappSession.gob")
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(session)
	if err != nil {
		return err
	}
	return nil
}
