
# RateLimitingConfig


## Properties

Name | Type
------------ | -------------
`burst` | number
`enabled` | boolean
`requestsPerMinute` | number

## Example

```typescript
import type { RateLimitingConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "burst": null,
  "enabled": null,
  "requestsPerMinute": null,
} satisfies RateLimitingConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as RateLimitingConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


