package init

import "avyos.dev/pkg/connect"

const (
	ObjectId    = 1
	ListEvent   = 2
	StatusEvent = 3
)

type Init struct {
	conn *connect.Connection
}

func NewInit() (*Init, error) {
	conn, err := connect.SystemBus()
	if err != nil {
		return nil, err
	}
	return &Init{conn}, nil
}

func (o *Init) List(filter string) ([]string, error) {
	req, err := connect.Encode(filter)
	if err != nil {
		return nil, err
	}

	resp, err := o.conn.Call(ObjectId, ListEvent, req)

	if err != nil {
		return nil, err
	}
	return connect.Decode[[]string](resp)
}

func (o *Init) Status(service string) (string, error) {
	req, err := connect.Encode(service)
	if err != nil {
		return "", err
	}

	resp, err := o.conn.Call(ObjectId, StatusEvent, req)
	if err != nil {
		return "", err
	}
	return connect.Decode[string](resp)
}
