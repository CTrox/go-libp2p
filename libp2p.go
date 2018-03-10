package libp2p

import (
	"context"
	"fmt"
	"net"

	config "github.com/libp2p/go-libp2p/config"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"

	circuit "github.com/libp2p/go-libp2p-circuit"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	ifconnmgr "github.com/libp2p/go-libp2p-interface-connmgr"
	pnet "github.com/libp2p/go-libp2p-interface-pnet"
	metrics "github.com/libp2p/go-libp2p-metrics"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	secio "github.com/libp2p/go-libp2p-secio"
	filter "github.com/libp2p/go-maddr-filter"
	tcp "github.com/libp2p/go-tcp-transport"
	ws "github.com/libp2p/go-ws-transport"
	ma "github.com/multiformats/go-multiaddr"
	mplex "github.com/whyrusleeping/go-smux-multiplex"
	yamux "github.com/whyrusleeping/go-smux-yamux"
)

// Config describes a set of settings for a libp2p node
type Config = config.Config

// Option is a libp2p config option that can be given to the libp2p constructor
// (`libp2p.New`).
type Option = config.Option

// ChainOptions chains multiple options into a single option.
func ChainOptions(opts ...Option) Option {
	return func(cfg *Config) error {
		for _, opt := range opts {
			if err := opt(cfg); err != nil {
				return err
			}
		}
		return nil
	}
}

// New constructs a new libp2p node with the given options.
//
// Canceling the passed context will stop the returned libp2p node.
func New(ctx context.Context, opts ...Option) (host.Host, error) {
	var cfg Config
	if err := cfg.Apply(opts...); err != nil {
		return nil, err
	}
	return cfg.NewNode(ctx)
}

// ListenAddrStrings configures libp2p to listen on the given (unparsed)
// addresses.
func ListenAddrStrings(s ...string) Option {
	return func(cfg *Config) error {
		for _, addrstr := range s {
			a, err := ma.NewMultiaddr(addrstr)
			if err != nil {
				return err
			}
			cfg.ListenAddrs = append(cfg.ListenAddrs, a)
		}
		return nil
	}
}

// ListenAddrs configures libp2p to listen on the given addresses.
func ListenAddrs(addrs ...ma.Multiaddr) Option {
	return func(cfg *Config) error {
		cfg.ListenAddrs = append(cfg.ListenAddrs, addrs...)
		return nil
	}
}

// DefaultSecurity is the default security option.
//
// Useful when you want to extend, but not replace, the supported transport
// security protocols.
var DefaultSecurity = Security(secio.ID, secio.New)

// NoSecurity is an option that completely disables all transport security.
// It's incompatible with all other transport security protocols.
var NoSecurity Option = func(cfg *Config) error {
	if len(cfg.SecurityTransports) > 0 {
		return fmt.Errorf("cannot use security transports with an insecure libp2p configuration")
	}
	cfg.Insecure = true
	return nil
}

// Security configures libp2p to use the given security transport (or transport
// constructor).
//
// Name is the protocol name.
//
// The transport can be a constructed security.Transport or a function taking
// any subset of this libp2p node's:
// * Public key
// * Private key
// * Peer ID
// * Host
// * Network
// * Peerstore
func Security(name string, tpt interface{}) Option {
	return func(cfg *Config) error {
		if cfg.Insecure {
			return fmt.Errorf("cannot use security transports with an insecure libp2p configuration")
		}
		stpt, err := config.SecurityConstructor(tpt)
		if err == nil {
			cfg.SecurityTransports = append(cfg.SecurityTransports, config.MsSecC{SecC: stpt, ID: name})
		}
		return err
	}
}

// DefaultMuxer configures libp2p to use the stream connection multiplexers.
//
// Use this option when you want to *extend* the set of multiplexers used by
// libp2p instead of replacing them.
var DefaultMuxer = ChainOptions(
	Muxer("/yamux/1.0.0", yamux.DefaultTransport),
	Muxer("/mplex/6.3.0", mplex.DefaultTransport),
)

// Muxer configures libp2p to use the given stream multiplexer (or stream
// multiplexer constructor).
//
// Name is the protocol name.
//
// The transport can be a constructed mux.Transport or a function taking any
// subset of this libp2p node's:
// * Peer ID
// * Host
// * Network
// * Peerstore
func Muxer(name string, tpt interface{}) Option {
	return func(cfg *Config) error {
		mtpt, err := config.MuxerConstructor(tpt)
		if err == nil {
			cfg.Muxers = append(cfg.Muxers, config.MsMuxC{MuxC: mtpt, ID: name})
		}
		return err
	}
}

// Transport configures libp2p to use the given transport (or transport
// constructor).
//
// The transport can be a constructed transport.Transport or a function taking
// any subset of this libp2p node's:
// * Transport Upgrader (*tptu.Upgrader)
// * Host
// * Stream muxer (muxer.Transport)
// * Security transport (security.Transport)
// * Private network protector (pnet.Protector)
// * Peer ID
// * Private Key
// * Public Key
// * Address filter (filter.Filter)
// * Peerstore
func Transport(tpt interface{}) Option {
	return func(cfg *Config) error {
		tptc, err := config.TransportConstructor(tpt)
		if err == nil {
			cfg.Transports = append(cfg.Transports, tptc)
		}
		return err
	}
}

// DefaultTransports are the default libp2p transports.
//
// Use this option when you want to *extend* the set of multiplexers used by
// libp2p instead of replacing them.
var DefaultTransports = ChainOptions(
	Transport(tcp.NewTCPTransport),
	Transport(ws.New),
)

// Peerstore configures libp2p to use the given peerstore.
func Peerstore(ps pstore.Peerstore) Option {
	return func(cfg *Config) error {
		if cfg.Peerstore != nil {
			return fmt.Errorf("cannot specify multiple peerstore options")
		}

		cfg.Peerstore = ps
		return nil
	}
}

// PrivateNetwork configures libp2p to use the given private network protector.
func PrivateNetwork(prot pnet.Protector) Option {
	return func(cfg *Config) error {
		if cfg.Protector != nil {
			return fmt.Errorf("cannot specify multiple private network options")
		}

		cfg.Protector = prot
		return nil
	}
}

// BandwidthReporter configures libp2p to use the given bandwidth reporter.
func BandwidthReporter(rep metrics.Reporter) Option {
	return func(cfg *Config) error {
		if cfg.Reporter != nil {
			return fmt.Errorf("cannot specify multiple bandwidth reporter options")
		}

		cfg.Reporter = rep
		return nil
	}
}

// Identity configures libp2p to use the given private key to identify itself.
func Identity(sk crypto.PrivKey) Option {
	return func(cfg *Config) error {
		if cfg.PeerKey != nil {
			return fmt.Errorf("cannot specify multiple identities")
		}

		cfg.PeerKey = sk
		return nil
	}
}

// ConnectionManager configures libp2p to use the given connection manager.
func ConnectionManager(connman ifconnmgr.ConnManager) Option {
	return func(cfg *Config) error {
		if cfg.ConnManager != nil {
			return fmt.Errorf("cannot specify multiple connection managers")
		}
		cfg.ConnManager = connman
		return nil
	}
}

// AddrsFactory configures libp2p to use the given address factory.
func AddrsFactory(factory config.AddrsFactory) Option {
	return func(cfg *Config) error {
		if cfg.AddrsFactory != nil {
			return fmt.Errorf("cannot specify multiple address factories")
		}
		cfg.AddrsFactory = factory
		return nil
	}
}

// EnableRelay configures libp2p to enable the relay transport.
func EnableRelay(options ...circuit.RelayOpt) Option {
	return func(cfg *Config) error {
		cfg.Relay = true
		cfg.RelayOpts = options
		return nil
	}
}

// FilterAddresses configures libp2p to never dial nor accept connections from
// the given addresses.
func FilterAddresses(addrs ...*net.IPNet) Option {
	return func(cfg *Config) error {
		if cfg.Filters == nil {
			cfg.Filters = filter.NewFilters()
		}
		for _, addr := range addrs {
			cfg.Filters.AddDialFilter(addr)
		}
		return nil
	}
}

// NATPortMap configures libp2p to use the default NATManager. The default
// NATManager will attempt to punch holes in your NAT.
func NATPortMap() Option {
	return NATManager(bhost.NewNATManager)
}

// NATManager will configure libp2p to use the requested NATManager. This
// function should be passed a NATManager *constructor* that takes a libp2p Network.
func NATManager(nm config.NATManagerC) Option {
	return func(cfg *Config) error {
		if cfg.NATManager != nil {
			return fmt.Errorf("cannot specify multiple NATManagers")
		}
		cfg.NATManager = nm
		return nil
	}
}
