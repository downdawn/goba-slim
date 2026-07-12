local refresh_key = ARGV[1] .. ARGV[2]
local raw_token = redis.call('GET', refresh_key)
if not raw_token then
  return {0, ''}
end

local token = cjson.decode(raw_token)
local session_key = ARGV[3] .. token.session_id
if token.state == 'spent' then
  local raw_session = redis.call('GET', session_key)
  if raw_session then
    local session = cjson.decode(raw_session)
    redis.call('DEL', ARGV[1] .. session.current_digest)
  end
  redis.call('DEL', session_key)
  redis.call('SREM', ARGV[4] .. token.user_id, token.session_id)
  return {-1, ''}
end

local raw_session = redis.call('GET', session_key)
if not raw_session then
  return {0, ''}
end
local session = cjson.decode(raw_session)
if session.current_digest ~= ARGV[2] then
  return {0, ''}
end

token.state = 'spent'
redis.call('SET', refresh_key, cjson.encode(token), 'PX', ARGV[7])
session.current_digest = ARGV[5]
session.expires_at = ARGV[8]
local new_token = {session_id = token.session_id, family_id = token.family_id, user_id = token.user_id, state = 'active'}
redis.call('SET', ARGV[1] .. ARGV[5], cjson.encode(new_token), 'PX', ARGV[7])
redis.call('SET', session_key, cjson.encode(session), 'PX', ARGV[7])
redis.call('PEXPIRE', ARGV[4] .. token.user_id, ARGV[7])
return {1, cjson.encode(session)}
