package main

import (
	"context"
	"flag"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"golang.org/x/sys/unix"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	var (
		addr       = flag.String("addr", "localhost:6379", "Socket address of the Redis server in host:port notation.")
		keyPrefix  = flag.String("key-prefix", "junk-", "Prefix for keys written by this program.")
		expiration = flag.Duration("expiration", time.Hour, "Redis SETEX expiration duration for keys written by this program.")
	)

	flag.Parse()

	client := redis.NewClient(&redis.Options{
		Addr:       *addr,
		MaxRetries: 2,
	})

	opts := options{
		KeyPrefix:  *keyPrefix,
		Expiration: *expiration,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, unix.SIGINT, unix.SIGTERM)
	go func() {
		select {
		case <-ctx.Done():
		case s := <-signals:
			log.Printf("%s received - terminating...", s)
			cancel()
		}
	}()

	if err := junk(ctx, client, &opts); err != nil {
		log.Fatal(err)
	}
}

type options struct {
	KeyPrefix   string
	Expiration  time.Duration
	NumKeys     uint
	ValueLength uint
	Parallelism uint
}

func setDefaultOptions(opts *options) *options {
	o := *opts
	if o.NumKeys == 0 {
		o.NumKeys = 51200
	}
	if o.ValueLength == 0 {
		o.ValueLength = 10240
	}
	if o.Parallelism == 0 {
		o.Parallelism = 4
	}
	return &o
}

func junk(ctx context.Context, client *redis.Client, opts *options) error {
	opts = setDefaultOptions(opts)

	var (
		shares   = keyShares(opts.NumKeys, opts.Parallelism)
		wg       = sync.WaitGroup{}
		mtx      sync.Mutex
		firstErr error
	)

	setErr := func(err error) {
		if err == nil {
			return
		}
		mtx.Lock()
		defer mtx.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}

	work := func(keys uint) {
		defer wg.Done()
		for i := uint(0); i < keys; i++ {
			select {
			case <-ctx.Done():
				setErr(ctx.Err())
				return
			default:
			}

			k := generateKey(opts.KeyPrefix)
			v := generateValue(opts.ValueLength)
			if err := client.Set(k, v, opts.Expiration).Err(); err != nil {
				setErr(err)
				return
			}
		}
	}

	for i := uint(0); i < opts.Parallelism; i++ {
		wg.Add(1)
		go work(shares[i])
	}
	wg.Wait()

	return firstErr
}

func keyShares(keys, workers uint) []uint {
	s := keys / workers
	r := keys % workers
	shares := make([]uint, workers)
	for i := uint(0); i < workers; i++ {
		shares[i] = s
	}
	shares[workers-1] += r
	return shares
}

func generateKey(prefix string) string {
	return prefix + randomString(32)
}

func generateValue(length uint) string {
	return randomString(length)
}

var alphabet string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(length uint) string {
	s := make([]byte, length)
	for i := uint(0); i < length; i++ {
		s[i] = alphabet[rand.Int()%len(alphabet)]
	}
	return string(s)
}
