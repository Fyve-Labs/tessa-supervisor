package remote_commands

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"

	"github.com/Fyve-Labs/tessa-daemon/internal/config"
	"github.com/Fyve-Labs/tessa-daemon/internal/tunnel"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/pocketbase/pocketbase/tools/store"
)

const NatsCommandsSubject = "tessa.devices.%s.commands.json"

type CommandManager struct {
	natsConn      *nats.Conn
	tunnelManager *tunnel.Manager
	subscriptions []*nats.Subscription
	commands      *store.Store[string, *Command] // Thread-safe store of active commands
}

func NewCommandManager(natsConn *nats.Conn, tunnelManager *tunnel.Manager) *CommandManager {
	return &CommandManager{
		commands:      store.New(map[string]*Command{}),
		subscriptions: make([]*nats.Subscription, 0),
		natsConn:      natsConn,
		tunnelManager: tunnelManager,
	}
}

// GetCommand returns a system by ID from the store
func (cm *CommandManager) GetCommand(commandID string) (*Command, error) {
	cmd, ok := cm.commands.GetOk(commandID)
	if !ok {
		return nil, fmt.Errorf("command not found")
	}
	return cmd, nil
}

func (cm *CommandManager) Initialize() error {
	// start nat subscription
	if err := cm.startSubscriptions(); err != nil {
		return err
	}

	// Load commands from config
	//_ = cm.AddCommand(&Command{
	//	ID: "start-ssh",
	//})

	_ = cm.AddCommand(&Command{
		ID: "enable-beszel",
		Payload: &BeszelConfig{
			ServerURL:    "http://192.168.1.100:8090",
			Token:        "",
			SSHPublicKey: "",
		},
	})

	return nil
}

func (cm *CommandManager) AddCommand(cmd *Command) error {
	if cm.commands.Has(cmd.ID) {
		slog.Info("command is already running", slog.String("command", cmd.ID))
		return nil
	}

	cmd.manager = cm
	cmd.ctx, cmd.cancel = cmd.getContext()
	cm.commands.Set(cmd.ID, cmd)

	go cmd.Start()

	return nil
}

func (cm *CommandManager) RemoveCommand(commandID string) error {
	command, ok := cm.commands.GetOk(commandID)
	if !ok {
		return errors.New("command not found")
	}

	// Stop the update goroutine
	if command.cancel != nil {
		command.cancel()
	}

	cm.commands.Remove(commandID)
	return nil
}

func (cm *CommandManager) startSubscriptions() error {
	sub, err := cm.natsConn.Subscribe(fmt.Sprintf(NatsCommandsSubject, config.DeviceName), func(m *nats.Msg) {

		var req CommandRequest
		if err := json.Unmarshal(m.Data, &req); err != nil {
			log.Printf("ERROR: Could not unmarshal command request: %v", err)
			return
		}

		slog.Info("Received command request", slog.String("command", req.Command))
		err := cm.AddCommand(&Command{
			ID:      req.Command,
			Payload: req.Payload,
		})

		if err != nil {
			log.Printf("AddCommand: %v", err)
		}
	})

	if err != nil {
		return err
	}

	cm.subscriptions = append(cm.subscriptions, sub)

	return nil
}

func (cm *CommandManager) Stop() error {
	for _, sub := range cm.subscriptions {
		if err := sub.Drain(); err != nil {
			return err
		}
	}

	for _, cmd := range cm.commands.GetAll() {
		if err := cm.RemoveCommand(cmd.ID); err != nil {
			return err
		}
	}

	return nil
}
