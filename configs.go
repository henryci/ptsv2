package main

/*****************
 * GLOBAL CONFIG *
 *****************/

// MaxFileSize is the maximum filesize (in bytes). This is used to limit the buffer
var MaxFileSize = 16 * 1024

// MaxResponseDelay is the max time in seconds to wait before responding to a post
var MaxResponseDelay = 5

// MaxDumpsInToilet is a count of how many dumps a toilet can have before it starts deleting to make room for new ones
var MaxDumpsInToilet = 240

// NumDumpsToDelete tells the app how many dumps to delete when it is deleteing dumps
var NumDumpsToDelete = 50

// MinSecondsBetweenDeletes is the minimum time between hitting delete limits before a toilet gets clogged
var MinSecondsBetweenDeletes float64 = 180
