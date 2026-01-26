#!/bin/bash

GO_MODULE="github.com/GuanceCloud/cliutils"

# 1. Define Standard Imports
# We include the current directory (.) so batch.proto can find point/point.proto
INCLUDES="-I=. -I=.. -I=${GOPATH}/src -I=${GOPATH}/src/github.com/gogo/protobuf/protobuf"

# 2. Define Mappings (The Critical Part)
# This tells gogoslick: "When you see an import for point/point.proto,
# use the Go package github.com/your-org/repo/point in the generated code."
# REPLACE 'github.com/your-org/repo' with your actual module name.
MAPPINGS="Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types,\
Mpoint/point.proto=${GO_MODULE}/point"

# 3. Build batch.proto
echo "Building batch.proto & tracedata.proto"
protoc $INCLUDES \
    --gogoslick_out="${MAPPINGS}:." \
    aggregate/batch.proto \
    aggregate/tracedata.proto

echo "Done."
