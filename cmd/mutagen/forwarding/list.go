package forwarding

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/spf13/cobra"

	"github.com/fatih/color"

	"github.com/havoc-io/mutagen/cmd/mutagen/daemon"
	forwardingpkg "github.com/havoc-io/mutagen/pkg/forwarding"
	"github.com/havoc-io/mutagen/pkg/grpcutil"
	"github.com/havoc-io/mutagen/pkg/selection"
	forwardingsvcpkg "github.com/havoc-io/mutagen/pkg/service/forwarding"
	urlpkg "github.com/havoc-io/mutagen/pkg/url"
)

func formatConnectionStatus(connected bool) string {
	if connected {
		return "Connected"
	}
	return "Disconnected"
}

func printEndpointStatus(name string, url *urlpkg.URL, connected bool) {
	// Print header.
	fmt.Printf("%s:\n", name)

	// Print URL.
	fmt.Println("\tURL:", url.Format("\n\t\t"))

	// Print connection status.
	fmt.Printf("\tConnection state: %s\n", formatConnectionStatus(connected))
}

func printSessionStatus(state *forwardingpkg.State) {
	// Print status.
	statusString := state.Status.Description()
	if state.Session.Paused {
		statusString = color.YellowString("[Paused]")
	}
	fmt.Fprintln(color.Output, "Status:", statusString)

	// Print the last error, if any.
	if state.LastError != "" {
		color.Red("Last error: %s\n", state.LastError)
	}
}

func listMain(command *cobra.Command, arguments []string) error {
	// Create session selection specification.
	selection := &selection.Selection{
		All:            len(arguments) == 0 && listConfiguration.labelSelector == "",
		Specifications: arguments,
		LabelSelector:  listConfiguration.labelSelector,
	}
	if err := selection.EnsureValid(); err != nil {
		return errors.Wrap(err, "invalid session selection specification")
	}

	// Connect to the daemon and defer closure of the connection.
	daemonConnection, err := daemon.CreateClientConnection(true)
	if err != nil {
		return errors.Wrap(err, "unable to connect to daemon")
	}
	defer daemonConnection.Close()

	// Create a session service client.
	sessionService := forwardingsvcpkg.NewForwardingClient(daemonConnection)

	// Invoke list.
	request := &forwardingsvcpkg.ListRequest{
		Selection: selection,
	}
	response, err := sessionService.List(context.Background(), request)
	if err != nil {
		return errors.Wrap(grpcutil.PeelAwayRPCErrorLayer(err), "list failed")
	} else if err = response.EnsureValid(); err != nil {
		return errors.Wrap(err, "invalid list response received")
	}

	// Loop through and print sessions.
	for _, state := range response.SessionStates {
		fmt.Println(delimiterLine)
		printSession(state)
		printEndpointStatus("Source", state.Session.Source, state.SourceConnected)
		printEndpointStatus("Destination", state.Session.Destination, state.DestinationConnected)
		printSessionStatus(state)
	}

	// Print a final delimiter line if there were any sessions.
	if len(response.SessionStates) > 0 {
		fmt.Println(delimiterLine)
	}

	// Success.
	return nil
}

var listCommand = &cobra.Command{
	Use:          "list [<session>...]",
	Short:        "List existing forwarding sessions and their statuses",
	RunE:         listMain,
	SilenceUsage: true,
}

var listConfiguration struct {
	// help indicates whether or not help information should be shown for the
	// command.
	help bool
	// labelSelector encodes a label selector to be used in identifying which
	// sessions should be paused.
	labelSelector string
}

func init() {
	// Grab a handle for the command line flags.
	flags := listCommand.Flags()

	// Disable alphabetical sorting of flags in help output.
	flags.SortFlags = false

	// Manually add a help flag to override the default message. Cobra will
	// still implement its logic automatically.
	flags.BoolVarP(&listConfiguration.help, "help", "h", false, "Show help information")

	// Wire up list flags.
	flags.StringVar(&listConfiguration.labelSelector, "label-selector", "", "List sessions matching the specified label selector")
}