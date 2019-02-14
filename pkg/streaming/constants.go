package streaming

// Apparently this is an optimal chunk size according to https://github.com/grpc/grpc.github.io/issues/371
const ChunkSize = 64 * 1024

const Threshold = 4 * 1024 * 1024
