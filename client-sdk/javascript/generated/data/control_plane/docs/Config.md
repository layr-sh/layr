
# Config

Config schema

## Properties

Name | Type
------------ | -------------
`cache` | [CacheConfig](CacheConfig.md)
`cors` | [CORSConfig](CORSConfig.md)
`graphql` | [GraphQLConfig](GraphQLConfig.md)
`rateLimiting` | [RateLimitingConfig](RateLimitingConfig.md)
`realtime` | [RealtimeConfig](RealtimeConfig.md)
`rest` | [RESTConfig](RESTConfig.md)
`schemas` | Array&lt;string&gt;

## Example

```typescript
import type { Config } from ''

// TODO: Update the object below with actual values
const example = {
  "cache": null,
  "cors": null,
  "graphql": null,
  "rateLimiting": null,
  "realtime": null,
  "rest": null,
  "schemas": null,
} satisfies Config

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Config
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


