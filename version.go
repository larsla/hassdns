//go:generate sh -c "sed -i \"s|var Version = \\\".*\\\"|var Version = \\\"`echo $VERSION`\\\"|\" version.go"

package hassdns

// Version number
var Version = ""
