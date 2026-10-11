
# GetManifestResponse

GetManifestResponse schema

## Properties

Name | Type
------------ | -------------
`enabledServices` | Array&lt;string&gt;
`project` | [ManifestProjectInfo](ManifestProjectInfo.md)
`publishableKey` | string
`server` | [ManifestServerInfo](ManifestServerInfo.md)

## Example

```typescript
import type { GetManifestResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "enabledServices": null,
  "project": null,
  "publishableKey": null,
  "server": null,
} satisfies GetManifestResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as GetManifestResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


