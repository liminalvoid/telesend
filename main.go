package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gotd/contrib/bg"
	"github.com/gotd/td/examples"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/html"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/h2non/filetype"
	"github.com/pelletier/go-toml/v2"
)

func main() {
	filepathPtr := flag.String("path", "foo", "path to file")
	msgPtr := flag.String("msg", "", "message to the recipient")

	flag.Parse()

	// Read of create config if it doesn't exist
	cfg := &Config{}
	if err := readConfig(cfg); errors.Is(err, os.ErrNotExist) {
		r := bufio.NewReader(os.Stdin)

		fmt.Print("Enter app ID: ")
		appId, _ := r.ReadString('\n')
		fmt.Print("Enter app hash: ")
		appHash, _ := r.ReadString('\n')
		fmt.Print("Enter phone number: ")
		phoneNumber, _ := r.ReadString('\n')

		cfg.Auth.AppID, err = strconv.Atoi(appId[0 : len(appId)-1])
		if err != nil {
			fmt.Println("Invalid app ID: cannot convert string to int")

			return
		}
		cfg.Auth.AppHash = appHash[0 : len(appHash)-1]
		cfg.Auth.PhoneNumber = phoneNumber[0 : len(phoneNumber)-1]

		err = createConfig(cfg)
		if err != nil {
			panic(err)
		}
	} else if errors.Is(err, &toml.DecodeError{}) {
		fmt.Println("Error parsing config file")

		return
	} else if err != nil {
		panic(fmt.Errorf("unexpected error: %w", err))
	}

	// Create session storage file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	sessionDir := filepath.Join(homeDir, cfgFolder, "session")
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		panic(err)
	}
	sessionStorage := &telegram.FileSessionStorage{
		Path: filepath.Join(sessionDir, "session.json"),
	}

	// Initialize client
	options := telegram.Options{
		SessionStorage: sessionStorage,
		NoUpdates:      true,
	}
	client := telegram.NewClient(
		cfg.Auth.AppID,
		cfg.Auth.AppHash,
		options,
	)
	api := tg.NewClient(client)
	authFlow := auth.NewFlow(
		examples.Terminal{
			PhoneNumber: cfg.Auth.PhoneNumber,
		},
		auth.SendCodeOptions{},
	)

	// Connect client
	stop, err := bg.Connect(client)
	if err != nil {
		panic(err)
	}
	defer stop()

	ctx := context.Background()

	// Complete authentication
	if err := client.Auth().IfNecessary(ctx, authFlow); err != nil {
		panic(err)
	} else {
		fmt.Println("Successful authorization")
	}

	// Get all dialogs
	dialogs, err := api.MessagesGetDialogs(
		ctx,
		&tg.MessagesGetDialogsRequest{
			OffsetDate:    0,
			OffsetID:      0,
			OffsetPeer:    &tg.InputPeerEmpty{},
			Limit:         100,
			ExcludePinned: false,
		})
	if err != nil {
		panic(err)
	}

	// Get user's data
	users, err := api.UsersGetUsers(ctx, []tg.InputUserClass{&tg.InputUserSelf{}})
	if err != nil {
		panic(err)
	}
	me, ok := users[0].(*tg.User)
	if !ok {
		panic("unexpected type")
	}

	// Get user's full name
	selfFullName := fmt.Sprintf("%s %s (you)", me.FirstName, me.LastName)
	if me.LastName == "" {
		selfFullName = fmt.Sprintf("%s (you)", me.FirstName)
	}

	chats := make(map[string]string, 0)
	chats[selfFullName] = me.Username
	choices := make([]string, 0)
	choices = append(choices, selfFullName)

	// Iterate through dialogs and get data
	switch d := dialogs.(type) {
	case *tg.MessagesDialogsSlice:
		for _, dialog := range d.Dialogs {
			switch peer := dialog.GetPeer().(type) {
			case *tg.PeerUser:
				result, err := api.MessagesGetPeerDialogs(
					ctx,
					[]tg.InputDialogPeerClass{
						&tg.InputDialogPeer{
							Peer: &tg.InputPeerUser{UserID: peer.UserID},
						},
					},
				)
				if err != nil {
					fmt.Println(err)
				}

				for _, user := range result.Users {
					switch u := user.(type) {
					case *tg.User:
						if u.ID == me.ID {
							continue
						}

						fullName := fmt.Sprintf("%s %s", u.FirstName, u.LastName)

						if u.Username == "" {
							chats[fullName] = u.Phone
						} else {
							chats[fullName] = u.Username
						}

						choices = append(choices, fullName)
					}
				}
			case *tg.PeerChat:
				result, err := api.MessagesGetPeerDialogs(
					ctx,
					[]tg.InputDialogPeerClass{
						&tg.InputDialogPeer{
							Peer: &tg.InputPeerChat{ChatID: peer.ChatID},
						},
					},
				)
				if err != nil {
					fmt.Println(err)
				}

				for _, chat := range result.Chats {
					switch c := chat.(type) {
					case *tg.Chat:
						chats[c.Title] = fmt.Sprintf("%d", c.ID)
						choices = append(choices, c.Title)
					}
				}
			}
		}
	default:
		fmt.Println("Unknown dialog type")
	}

	cfgMisc := &Config{
		General: cfg.General,
		Misc:    cfg.Misc,
	}

	// Add UI
	p := tea.NewProgram(newModel(choices, cfgMisc))
	m, err := p.Run()
	if err != nil {
		panic(err)
	}
	mod, ok := m.(UiModel)
	if !ok {
		panic("something went wrong :(")
	}

	// Helper for uploading
	u := uploader.NewUploader(api)
	sender := message.NewSender(api).WithUploader(u)

	// Get filename
	path, err := filepath.Abs(*filepathPtr)
	if err != nil {
		panic(err)
	}
	filename := filepath.Base(*filepathPtr)

	// Read file into buffer
	buf, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	// Get filetype
	kind, err := filetype.Match(buf)
	if err != nil {
		panic(err)
	}

	// Upload file
	upload, err := u.FromBytes(ctx, filename, buf)
	if err != nil {
		panic(err)
	}

	// Set filename, message and filetype
	document := message.
		UploadedDocument(upload, html.String(nil, *msgPtr)).
		MIME(kind.MIME.Value).
		Filename(filename)
	if filetype.IsVideo(buf) {
		document.Video()
	} else if filetype.IsAudio(buf) {
		document.Audio()
	}

	// Send to selected recipients
	for _, recipient := range mod.Choices {
		if recipient == "" {
			continue
		}

		contact := chats[recipient]

		target := sender.Resolve(contact)
		if _, err := target.Media(ctx, document); err != nil {
			fmt.Println("error: ", err)
		}
	}
}
