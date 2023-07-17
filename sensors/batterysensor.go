package sensors

import (
	"context"
	"errors"
	"github.com/bluenviron/goroslib/v2"
	"github.com/edaniels/golog"
	"github.com/shawnbmccarthy/viam-ros-module/pkg/msgs/transbot_msgs"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
	"strings"
	"sync"
)

var BatteryModel = resource.NewModel("viamlabs", "ros", "yahboombattery")

type BatterySensor struct {
	resource.Named

	mu         sync.Mutex
	nodeName   string
	primaryUri string
	topic      string
	node       *goroslib.Node
	subscriber *goroslib.Subscriber
	msg        *transbot_msgs.Battery
	logger     golog.Logger
}

func init() {
	resource.RegisterComponent(
		sensor.API,
		BatteryModel,
		resource.Registration[sensor.Sensor, *BatterySensorConfig]{
			Constructor: NewBatterySensor,
		},
	)
}

func NewBatterySensor(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (sensor.Sensor, error) {
	b := &BatterySensor{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	if err := b.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return b, nil
}

func (b *BatterySensor) Reconfigure(
	_ context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodeName = conf.Attributes.String("node_name")
	b.primaryUri = conf.Attributes.String("primary_uri")
	b.topic = conf.Attributes.String("topic")

	if len(strings.TrimSpace(b.primaryUri)) == 0 {
		return errors.New("ROS primary uri must be set to hostname:port")
	}

	if len(strings.TrimSpace(b.topic)) == 0 {
		return errors.New("ROS topic must be set to valid sensor topic")
	}

	if len(strings.TrimSpace(b.nodeName)) == 0 {
		b.nodeName = "viam_batterysensor_node"
	}

	if b.subscriber != nil {
		if b.subscriber.Close() != nil {
			b.logger.Warn("failed to close subscriber")
		}
	}

	if b.node != nil {
		if b.node.Close() != nil {
			b.logger.Warn("failed to close node")
		}
	}

	var err error
	b.node, err = goroslib.NewNode(goroslib.NodeConf{
		Name:          b.nodeName,
		MasterAddress: b.primaryUri,
	})

	if err != nil {
		return err
	}

	b.subscriber, err = goroslib.NewSubscriber(goroslib.SubscriberConf{
		Node:     b.node,
		Topic:    b.topic,
		Callback: b.processMessage,
	})

	if err != nil {
		return err
	}

	return nil
}

func (b *BatterySensor) processMessage(msg *transbot_msgs.Battery) {
	b.msg = msg
}

func (b *BatterySensor) Readings(
	_ context.Context,
	_ map[string]interface{},
) (map[string]interface{}, error) {
	if b.msg == nil {
		return nil, errors.New("battery message not prepared")
	}
	return map[string]interface{}{"voltage": b.msg.Voltage}, nil
}

func (b *BatterySensor) Close(_ context.Context) error {
	err := b.subscriber.Close()
	err = b.node.Close()
	return err
}
