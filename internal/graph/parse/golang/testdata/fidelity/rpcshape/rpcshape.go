// Package rpcshape has a method with the net/rpc handler shape in a package
// that does not use net/rpc.
package rpcshape

// Args is an argument.
type Args struct{ A int }

// Reply is a result.
type Reply struct{ B int }

// Svc is not registered with any RPC server.
type Svc struct{}

// Do copies the argument into the reply.
func (s *Svc) Do(args *Args, reply *Reply) error {
	reply.B = args.A
	return nil
}
