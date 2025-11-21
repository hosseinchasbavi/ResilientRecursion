package engine

import (
    "encoding/binary"
    "fmt"
    "hash/fnv"
    "math"
)

func HashFloat64(r float64) uint64 {
    return math.Float64bits(r)
}

func GetPodForR(rHash uint64, totalPods int) int {
    h := fnv.New32a()
    binary.Write(h, binary.LittleEndian, rHash)
    return int(h.Sum32() % uint32(totalPods))
}

func ParsePodID(podID string) int {
    var id int
    fmt.Sscanf(podID, "pod-%d", &id)
    return id
}