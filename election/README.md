## Election Library
This is a network agnostic implementation of the leader election portion of the RAFT protocol. This library provides no
peer discovery mechanism, as such the user of this library must call `SetPeers()` on the node when the list of peers
changes. Users can use any third party service discovery mechanism, such as consul, etc, k8s, or memberlist.
 
For our internal uses we choose https://github.com/hashicorp/memberlist such that our services have as few external
dependencies as possible.

### Usage
In order to use the library on the network the user must provide a `SendRPC()` function at initialization time. This
function will be called when RPC communication between `election.Node` is needed. A node that wishes to receive
an RPC call must in turn call `Node.ReceiveRPC()` when the RPC request is received by what ever network protocol the 
user implements.

You can see a simple example of this using http by looking at `example_test.go`