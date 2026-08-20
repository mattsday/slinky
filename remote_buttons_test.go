package main

// remoteButtonIDs are the button IDs apiCall forwards to a control backend
// today (see html/remote.html). volume-up/volume-down/mute are deliberately
// excluded - remote.js handles them client-side against the <video> element
// and they never reach a control backend. Shared between skyq_test.go and
// skystream_test.go so both control backends are checked against the same
// source of truth for "what buttons actually exist."
var remoteButtonIDs = []string{
	"power", "select", "return", "channel-up", "channel-down",
	"info", "search", "menu",
	"direction-up", "direction-down", "direction-left", "direction-right",
	"red", "green", "yellow", "blue",
	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
	"play", "rewind", "fast-forward", "record",
}
