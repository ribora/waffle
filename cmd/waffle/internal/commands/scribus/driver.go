package scribus

import "errors"

const driverPrefix = "gitlab.com/gorib/waffle/config/tools/scribus/"

// The mode is a flag with a default, the driver is not: a scaffolder that picked
// a broker would answer the one question this tool was rebuilt not to answer.
var errNoDriver = errors.New("--driver is required: it names the broker module under config/tools/scribus, e.g. --driver amqp")
