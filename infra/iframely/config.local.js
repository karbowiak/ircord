// Private metadata service. Go selects typed data and stores media separately.
export default {
  host: '0.0.0.0',
  port: 8061,
  DEBUG: false,
  RICH_LOG_ENABLED: false,
  CACHE_ENGINE: 'node-cache',
  CACHE_TTL: 3600,
  CACHE_ERROR_TTL: 60,
  API_REQUEST_CACHE_TTL: 3600,
  IMAGE_META_CACHE_TTL: 3600,
  RESPONSE_TIMEOUT: 10000,
};
