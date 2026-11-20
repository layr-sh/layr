
# GetStatsResponse

GetStatsResponse schema

## Properties

Name | Type
------------ | -------------
`averageDurationMs` | number
`bundleSizeBytes` | number
`clientErrorsLast24Hours` | number
`deployedEndpoints` | number
`domainRoutes` | number
`domainsNeedingTls` | number
`endpoints` | [Array&lt;EndpointUsage&gt;](EndpointUsage.md)
`executionsLast24Hours` | number
`failedExecutionsLast24Hours` | number
`generatedAt` | Date
`lastExecutedAt` | Date
`p95DurationMs` | number
`privateEndpoints` | number
`publicEndpoints` | number
`runtimes` | [{ [key: string]: RunnerHealth; }](RunnerHealth.md)
`serverErrorsLast24Hours` | number
`totalDeployments` | number
`totalDomains` | number
`totalEndpoints` | number
`totalExecutions` | number
`windowStartedAt` | Date

## Example

```typescript
import type { GetStatsResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "averageDurationMs": null,
  "bundleSizeBytes": null,
  "clientErrorsLast24Hours": null,
  "deployedEndpoints": null,
  "domainRoutes": null,
  "domainsNeedingTls": null,
  "endpoints": null,
  "executionsLast24Hours": null,
  "failedExecutionsLast24Hours": null,
  "generatedAt": null,
  "lastExecutedAt": null,
  "p95DurationMs": null,
  "privateEndpoints": null,
  "publicEndpoints": null,
  "runtimes": null,
  "serverErrorsLast24Hours": null,
  "totalDeployments": null,
  "totalDomains": null,
  "totalEndpoints": null,
  "totalExecutions": null,
  "windowStartedAt": null,
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


