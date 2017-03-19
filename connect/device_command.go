package connect

import (
	"github.com/gogo/protobuf/proto"
	"github.com/micromdm/micromdm/connect/internal/devicecommandproto"
	"github.com/pkg/errors"
)

type Command struct {
	UUID    string
	Payload []byte
}

type DeviceCommand struct {
	DeviceUDID string
	Commands   []Command
}

func MarshalDeviceCommand(c *DeviceCommand) ([]byte, error) {
	protoc := devicecommandproto.DeviceCommand{
		DeviceUdid: c.DeviceUDID,
	}
	for _, command := range c.Commands {
		protoc.Commands = append(protoc.Commands, &devicecommandproto.Command{
			Payload: command.Payload,
			Uuid:    command.UUID,
		})
	}
	return proto.Marshal(&protoc)
}

func UnmarshalDeviceCommand(data []byte, c *DeviceCommand) error {
	var pb devicecommandproto.DeviceCommand
	if err := proto.Unmarshal(data, &pb); err != nil {
		return errors.Wrap(err, "unmarshal proto to DeviceCommand")
	}
	c.DeviceUDID = pb.GetDeviceUdid()
	protoCommands := pb.GetCommands()
	for _, command := range protoCommands {
		c.Commands = append(c.Commands, Command{
			UUID:    command.GetUuid(),
			Payload: command.GetPayload(),
		})
	}
	return nil
}
