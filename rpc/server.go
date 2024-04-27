package rpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"

	"github.com/hyprspace/hyprspace/config"
	"github.com/hyprspace/hyprspace/p2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type HyprspaceRPC struct {
	host   host.Host
	config config.Config
}

func (hsr *HyprspaceRPC) Status(args *Args, reply *StatusReply) error {
	netPeersCurrent := 0
	var netPeerAddrsCurrent []string
	for _, p := range hsr.config.Peers {
		if hsr.host.Network().Connectedness(p.ID) == network.Connected {
			netPeersCurrent = netPeersCurrent + 1
			for _, c := range hsr.host.Network().ConnsToPeer(p.ID) {
				netPeerAddrsCurrent = append(netPeerAddrsCurrent, fmt.Sprintf("%s/p2p/%s (%s)",
					c.RemoteMultiaddr().String(),
					p.ID.String(),
					hsr.host.Peerstore().LatencyEWMA(p.ID).String(),
				))
			}
		}
	}
	var addrStrings []string
	for _, ma := range hsr.host.Addrs() {
		addrStrings = append(addrStrings, ma.String())
	}
	*reply = StatusReply{
		hsr.host.ID().String(),
		len(hsr.host.Network().Conns()),
		netPeersCurrent,
		netPeerAddrsCurrent,
		len(hsr.config.Peers),
		addrStrings,
	}
	return nil
}

func (hsr *HyprspaceRPC) Route(args *RouteArgs, reply *RouteReply) error {
	switch args.Action {
	case Show:
		var routes []RouteInfo
		for _, r := range hsr.config.Routes {
			connected := hsr.host.Network().Connectedness(r.Target.ID) == network.Connected
			reroute, found := p2p.FindReroute(r.Network, false)
			relayAddr := r.Target.ID
			if found {
				relayAddr = reroute.To
			}
			routes = append(routes, RouteInfo{
				Network:     r.Network,
				TargetAddr:  r.Target.ID,
				RelayAddr:   relayAddr,
				IsRelay:     found,
				IsConnected: connected,
			})
		}
		*reply = RouteReply{
			Routes: routes,
		}
	case Relay:
		if len(args.Args) != 2 {
			return errors.New("expected exactly 2 arguments")
		}
		var networks []net.IPNet
		if args.Args[0] == "all" {
			for _, r := range hsr.config.Routes {
				networks = append(networks, r.Network)
			}
		} else {
			_, network, err := net.ParseCIDR(args.Args[0])
			if err != nil {
				return err
			} else if _, found := config.FindRoute(hsr.config.Routes, *network); !found {
				return errors.New("no such network")
			}
			networks = []net.IPNet{*network}
		}
		p, err := peer.Decode(args.Args[1])
		if err != nil {
			return err
		} else if _, found := config.FindPeer(hsr.config.Peers, p); !found {
			return errors.New("no such peer")
		}
		for _, n := range networks {
			p2p.AddReroute(n, p)
		}
	case Reset:
		if len(args.Args) != 1 {
			return errors.New("expected exactly 1 argument")
		}
		var networks []net.IPNet
		if args.Args[0] == "all" {
			for _, r := range hsr.config.Routes {
				networks = append(networks, r.Network)
			}
		} else {
			_, network, err := net.ParseCIDR(args.Args[0])
			if err != nil {
				return err
			} else if _, found := config.FindRoute(hsr.config.Routes, *network); !found {
				return errors.New("no such network")
			}
			networks = []net.IPNet{*network}
		}
		for _, n := range networks {
			p2p.FindReroute(n, true)
		}
	default:
		return errors.New("no such action")
	}
	return nil
}

func (hsr *HyprspaceRPC) Peers(args *Args, reply *PeersReply) error {
	var peerAddrs []string
	for _, c := range hsr.host.Network().Conns() {
		peerAddrs = append(peerAddrs, fmt.Sprintf("%s/p2p/%s", c.RemoteMultiaddr().String(), c.RemotePeer().String()))
	}
	*reply = PeersReply{peerAddrs}
	return nil
}

func RpcServer(ctx context.Context, ma multiaddr.Multiaddr, host host.Host, config config.Config) {
	hsr := HyprspaceRPC{host, config}
	rpc.Register(&hsr)

	addr, err := ma.ValueForProtocol(multiaddr.P_UNIX)
	if err != nil {
		log.Fatal("[!] Failed to parse multiaddr: ", err)
	}
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "unix", addr)
	os.Chmod(addr, 0o0770)
	if err != nil {
		log.Fatal("[!] Failed to launch RPC server: ", err)
	}
	fmt.Println("[-] RPC server ready")
	go rpc.Accept(l)
	<-ctx.Done()
	fmt.Println("[-] Closing RPC server")
	l.Close()
}