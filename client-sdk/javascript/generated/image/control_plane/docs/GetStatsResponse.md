
# GetStatsResponse

GetStatsResponse schema

## Properties

Name | Type
------------ | -------------
`bytesServed` | number
`cacheHitRatio` | number
`cacheHits` | number
`cacheMisses` | number
`memoryUsageBytes` | number
`totalRequests` | number

## Example

```typescript
import type { GetStatsResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "bytesServed": null,
  "cacheHitRatio": null,
  "cacheHits": null,
  "cacheMisses": null,
  "memoryUsageBytes": null,
  "totalRequests": null,
} satisfies GetStatsResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as GetStatsResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


