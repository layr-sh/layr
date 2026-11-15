
# OIDCConfig


## Properties

Name | Type
------------ | -------------
`clients` | [Array&lt;OIDCClientConfig&gt;](OIDCClientConfig.md)
`enabled` | boolean
`resourceServers` | [Array&lt;ResourceServerConfig&gt;](ResourceServerConfig.md)
`ui` | [OIDCUIConfig](OIDCUIConfig.md)

## Example

```typescript
import type { OIDCConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "clients": null,
  "enabled": null,
  "resourceServers": null,
  "ui": null,
} satisfies OIDCConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as OIDCConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


