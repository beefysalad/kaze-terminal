package claudesetup

import "github.com/beefysalad/kaze-terminal/internal/orderedjson"

type node = orderedjson.Node

var parse = orderedjson.Parse
var scalar = orderedjson.Scalar
var object = orderedjson.Object
