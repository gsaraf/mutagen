package main

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/spf13/pflag"

	"github.com/golang/protobuf/proto"

	"github.com/havoc-io/mutagen/cmd"
	"github.com/havoc-io/mutagen/cmd/profile"
	"github.com/havoc-io/mutagen/pkg/sync"
)

const (
	snapshotFile = "snapshot_test"
	cacheFile    = "cache_test"
)

var usage = `scan_bench [-h|--help] [-p|--profile] [-i|--ignore=<pattern>] <path>
`

func main() {
	// Parse command line arguments.
	flagSet := pflag.NewFlagSet("scan_bench", pflag.ContinueOnError)
	flagSet.SetOutput(ioutil.Discard)
	var ignores []string
	var enableProfile bool
	flagSet.StringSliceVarP(&ignores, "ignore", "i", nil, "specify ignore paths")
	flagSet.BoolVarP(&enableProfile, "profile", "p", false, "enable profiling")
	if err := flagSet.Parse(os.Args[1:]); err != nil {
		if err == pflag.ErrHelp {
			fmt.Fprint(os.Stdout, usage)
			return
		} else {
			cmd.Fatal(errors.Wrap(err, "unable to parse command line"))
		}
	}
	arguments := flagSet.Args()
	if len(arguments) != 1 {
		cmd.Fatal(errors.New("invalid number of paths specified"))
	}
	path := arguments[0]

	// Print information.
	fmt.Println("Analyzing", path)

	// Create a snapshot without any cache. If requested, enable CPU and memory
	// profiling.
	var profiler *profile.Profile
	var err error
	if enableProfile {
		if profiler, err = profile.New("scan_cold"); err != nil {
			cmd.Fatal(errors.Wrap(err, "unable to create profiler"))
		}
	}
	start := time.Now()
	snapshot, preservesExecutability, cache, err := sync.Scan(path, sha1.New(), nil, ignores, sync.SymlinkMode_SymlinkPortable)
	if err != nil {
		cmd.Fatal(errors.Wrap(err, "unable to create snapshot"))
	} else if snapshot == nil {
		cmd.Fatal(errors.New("target doesn't exist"))
	}
	stop := time.Now()
	if enableProfile {
		if err = profiler.Finalize(); err != nil {
			cmd.Fatal(errors.Wrap(err, "unable to finalize profiler"))
		}
		profiler = nil
	}
	fmt.Println("Cold scan took", stop.Sub(start))
	fmt.Println("Root preserves executability:", preservesExecutability)

	// Create a snapshot with a cache. If requested, enable CPU and memory
	// profiling.
	if enableProfile {
		if profiler, err = profile.New("scan_warm"); err != nil {
			cmd.Fatal(errors.Wrap(err, "unable to create profiler"))
		}
	}
	start = time.Now()
	snapshot, preservesExecutability, _, err = sync.Scan(path, sha1.New(), cache, ignores, sync.SymlinkMode_SymlinkPortable)
	if err != nil {
		cmd.Fatal(errors.Wrap(err, "unable to create snapshot"))
	} else if snapshot == nil {
		cmd.Fatal(errors.New("target has been deleted since original snapshot"))
	}
	stop = time.Now()
	if enableProfile {
		if err = profiler.Finalize(); err != nil {
			cmd.Fatal(errors.Wrap(err, "unable to finalize profiler"))
		}
		profiler = nil
	}
	fmt.Println("Warm scan took", stop.Sub(start))
	fmt.Println("Root preserves executability:", preservesExecutability)

	// Serialize it.
	start = time.Now()
	serializedSnapshot, err := proto.Marshal(snapshot)
	if err != nil {
		cmd.Fatal(errors.Wrap(err, "unable to serialize snapshot"))
	}
	stop = time.Now()
	fmt.Println("Snapshot serialization took", stop.Sub(start))

	// Deserialize it.
	start = time.Now()
	deserializedSnapshot := &sync.Entry{}
	if err = proto.Unmarshal(serializedSnapshot, deserializedSnapshot); err != nil {
		cmd.Fatal(errors.Wrap(err, "unable to deserialize snapshot"))
	}
	stop = time.Now()
	fmt.Println("Snapshot deserialization took", stop.Sub(start))

	// Validate the deserialized snapshot.
	start = time.Now()
	if err = deserializedSnapshot.EnsureValid(); err != nil {
		cmd.Fatal(errors.Wrap(err, "deserialized snapshot invalid"))
	}
	stop = time.Now()
	fmt.Println("Snapshot validation took", stop.Sub(start))

	// Write the serialized snapshot to disk.
	start = time.Now()
	if err = ioutil.WriteFile(snapshotFile, serializedSnapshot, 0600); err != nil {
		cmd.Fatal(errors.Wrap(err, "unable to write snapshot"))
	}
	stop = time.Now()
	fmt.Println("Snapshot write took", stop.Sub(start))

	// Read the serialized snapshot from disk.
	start = time.Now()
	if _, err = ioutil.ReadFile(snapshotFile); err != nil {
		cmd.Fatal(errors.Wrap(err, "unable to read snapshot"))
	}
	stop = time.Now()
	fmt.Println("Snapshot read took", stop.Sub(start))

	// Wipe the temporary file.
	if err = os.Remove(snapshotFile); err != nil {
		cmd.Fatal(errors.Wrap(err, "unable to remove snapshot"))
	}

	// TODO: I'd like to add a stable serialization benchmark since that's what
	// we really care about (especially since it has to copy the entire entry
	// tree), but I also don't want to expose that machinery publicly.

	// Print other information.
	fmt.Println("Serialized snapshot size is", len(serializedSnapshot), "bytes")
	fmt.Println(
		"Original/deserialized snapshots equivalent?",
		deserializedSnapshot.Equal(snapshot),
	)

	// Checksum it.
	start = time.Now()
	sha1.Sum(serializedSnapshot)
	stop = time.Now()
	fmt.Println("SHA-1 snapshot digest took", stop.Sub(start))

	// TODO: I'd like to add a copy benchmark since copying is used in a lot of
	// our transformation functions, but I also don't want to expose this
	// function publicly.

	// Serialize the cache.
	start = time.Now()
	serializedCache, err := proto.Marshal(cache)
	if err != nil {
		cmd.Fatal(errors.Wrap(err, "unable to serialize cache"))
	}
	stop = time.Now()
	fmt.Println("Cache serialization took", stop.Sub(start))

	// Deserialize the cache.
	start = time.Now()
	deserializedCache := &sync.Cache{}
	if err = proto.Unmarshal(serializedCache, deserializedCache); err != nil {
		cmd.Fatal(errors.Wrap(err, "unable to deserialize cache"))
	}
	stop = time.Now()
	fmt.Println("Cache deserialization took", stop.Sub(start))

	// Write the serialized cache to disk.
	start = time.Now()
	if err = ioutil.WriteFile(cacheFile, serializedCache, 0600); err != nil {
		cmd.Fatal(errors.Wrap(err, "unable to write cache"))
	}
	stop = time.Now()
	fmt.Println("Cache write took", stop.Sub(start))

	// Read the serialized cache from disk.
	start = time.Now()
	if _, err = ioutil.ReadFile(cacheFile); err != nil {
		cmd.Fatal(errors.Wrap(err, "unable to read cache"))
	}
	stop = time.Now()
	fmt.Println("Cache read took", stop.Sub(start))

	// Wipe the temporary file.
	if err = os.Remove(cacheFile); err != nil {
		cmd.Fatal(errors.Wrap(err, "unable to remove cache"))
	}

	// Print other information.
	fmt.Println("Serialized cache size is", len(serializedCache), "bytes")
}
