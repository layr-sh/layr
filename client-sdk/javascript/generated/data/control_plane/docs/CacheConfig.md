
# CacheConfig


## Properties

Name | Type
------------ | -------------
`catalogTtlSeconds` | number
`enabled` | boolean
`invalidateOnCdc` | boolean
`invalidateOnMutation` | boolean
`maxCachedQueries` | number
`queryTtlSeconds` | number

## Example

```typescript
import type { CacheConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "catalogTtlSeconds": null,
  "enabled": null,
  "invalidateOnCdc": null,
  "invalidateOnMutation": null,
  "maxCachedQueries": null,
  "queryTtlSeconds": null,
} satisfies CacheConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as CacheConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


