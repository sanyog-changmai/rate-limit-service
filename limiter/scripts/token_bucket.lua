-- KEYS[1] = bucket key e.g. "tb:abc123:user-service:/api/users"
-- ARGV[1] = rate (tokens/sec)
-- ARGV[2] = burst (max capacity)
-- ARGV[3] = current time in microseconds (from Redis TIME)

local rate  = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now   = tonumber(ARGV[3])

local data        = redis.call('HMGET', KEYS[1], 'tokens', 'last_refill_us')
local tokens      = tonumber(data[1]) or burst
local last_refill = tonumber(data[2]) or now

local elapsed    = (now - last_refill) / 1000000
local new_tokens = math.min(burst, tokens + elapsed * rate)

if new_tokens < 1 then
    local retry_after = (1 - new_tokens) / rate
    return {0, 0, burst, retry_after}
end

new_tokens = new_tokens - 1
redis.call('HMSET', KEYS[1], 'tokens', new_tokens, 'last_refill_us', now)
redis.call('EXPIRE', KEYS[1], math.ceil(burst / rate) * 2)

return {1, math.floor(new_tokens), burst, 0}
