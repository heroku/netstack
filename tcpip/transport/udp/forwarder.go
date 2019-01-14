package udp

import (
	"github.com/google/netstack/tcpip"
	"github.com/google/netstack/tcpip/buffer"
	"github.com/google/netstack/tcpip/stack"
	"github.com/google/netstack/waiter"
)

// Forwarder is a stream request forwarder, which allows clients what to do
// with a stream request, for example: ignore it, or process it.
//
// The canonical way of using it is to pass the Forwarder.HandlePacket function
// to stack.SetTransportProtocolHandler.
type Forwarder struct {
	handler func(*ForwarderRequest)

	stack *stack.Stack
}

// NewForwarder allocates and initializes a new forwarder.
func NewForwarder(s *stack.Stack, handler func(*ForwarderRequest)) *Forwarder {
	return &Forwarder{
		stack:   s,
		handler: handler,
	}
}

// HandlePacket handles all packets.
//
// This function is expected to be passed as an argument to the
// stack.SetTransportProtocolHandler function.
func (f *Forwarder) HandlePacket(r *stack.Route, id stack.TransportEndpointID, vv buffer.VectorisedView) bool {
	f.handler(&ForwarderRequest{
		stack: f.stack,
		route: r,
		id:    id,
		vv:    vv,
	})

	return true
}

// ForwarderRequest represents a stream request received by the forwarder and
// passed to the client. Clients may optionally create an endpoint to represent
// it via CreateEndpoint.
type ForwarderRequest struct {
	stack *stack.Stack
	route *stack.Route
	id    stack.TransportEndpointID
	vv    buffer.VectorisedView
}

// ID returns the 4-tuple (src address, src port, dst address, dst port) that
// represents the stream request.
func (r *ForwarderRequest) ID() stack.TransportEndpointID {
	return r.id
}

// CreateEndpoint creates a connected UDP endpoint for the stream request.
func (r *ForwarderRequest) CreateEndpoint(queue *waiter.Queue) (tcpip.Endpoint, *tcpip.Error) {
	ep := newEndpoint(r.stack, r.route.NetProto, queue)
	if err := r.stack.RegisterTransportEndpoint(r.route.NICID(), []tcpip.NetworkProtocolNumber{r.route.NetProto}, ProtocolNumber, r.id, ep, ep.reusePort); err != nil {
		ep.Close()
		return nil, err
	}

	ep.id = r.id
	ep.route = r.route.Clone()
	ep.dstPort = r.id.RemotePort
	ep.regNICID = r.route.NICID()

	ep.state = stateConnected

	ep.rcvMu.Lock()
	ep.rcvReady = true
	ep.rcvMu.Unlock()

	ep.HandlePacket(r.route, r.id, r.vv)

	return ep, nil
}
