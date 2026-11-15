
# RESTConfig


## Properties

Name | Type
------------ | -------------
`defaultLimit` | number
`enabled` | boolean
`excludedTables` | Array&lt;string&gt;
`maxLimit` | number

## Example

```typescript
import type { RESTConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "defaultLimit": null,
  "enabled": null,
  "excludedTables": null,
  "maxLimit": null,
} satisfies RESTConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as RESTConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


