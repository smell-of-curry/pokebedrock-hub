package internal

import "time"

// File permission constants
const (
	// DirectoryPermissions is the standard permission for creating directories
	DirectoryPermissions = 0755
)

// Game/Minecraft constants
const (
	// InventorySlots is the number of slots in a player's inventory
	InventorySlots = 36
	
	// MaxHealth is the maximum health value for a player
	MaxHealth = 20
	
	// MaxFood is the maximum food/hunger value for a player
	MaxFood = 20
	
	// DefaultWorldTime is the default time to set in the world (6000 = noon)
	DefaultWorldTime = 6000
	
	// WorldLoadRadius is the maximum radius to load around spawn
	WorldLoadRadius = 9999999
	
	// DefaultChunkLoaderCount is the number of chunk loaders to create
	DefaultChunkLoaderCount = 10
)

// Channel buffer size constants
const (
	// DefaultChannelBufferSize is the standard buffer size for channels
	DefaultChannelBufferSize = 100
	
	// SmallChannelBufferSize is used for smaller capacity channels
	SmallChannelBufferSize = 50
	
	// ProcessingBatchSize is the number of items to process per batch
	ProcessingBatchSize = 20
)

// Queue position thresholds
const (
	// HighPriorityQueueThreshold is the position threshold for "Almost your turn" message
	HighPriorityQueueThreshold = 3
	
	// MediumPriorityQueueThreshold is the position threshold for "Short wait" message  
	MediumPriorityQueueThreshold = 10
)

// Timeout constants (in milliseconds for shorter values, duration for longer ones)
const (
	// ShortRetryDelayMs is the delay for short retry operations
	ShortRetryDelayMs = 300
	
	// MinRandomDelayMs is the minimum random delay for spacing requests
	MinRandomDelayMs = 100
	
	// MaxRandomDelayMs is the maximum random delay for spacing requests (minus MinRandomDelayMs)
	MaxRandomDelayRangeMs = 1900
	
	// MinRandomDelayLongMs is the minimum random delay for longer operations
	MinRandomDelayLongMs = 500
	
	// MaxRandomDelayLongRangeMs is the maximum random delay range for longer operations
	MaxRandomDelayLongRangeMs = 2500
	
	// LongOperationTimeoutSec is the timeout for long operations in seconds
	LongOperationTimeoutSec = 30
	
	// DefaultTTL is the default TTL when parsing fails
	DefaultTTL = 60
)

// HTTP and network constants
const (
	// InternalServerError is the HTTP status code for server errors
	InternalServerError = 500
	
	// MaxHTTPRetries is the maximum number of HTTP retries
	MaxHTTPRetries = 1
)

// Duration constants for commonly used timeouts
const (
	// DefaultTimeout is used for standard operations
	DefaultTimeout = 5 * time.Second
	
	// SpeedEffectDuration is the duration for speed effects in lobby
	SpeedEffectDuration = 24 * time.Hour
)