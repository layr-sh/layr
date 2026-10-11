
# ThreatConfig


## Properties

Name | Type
------------ | -------------
`botProtection` | [BotProtectionConfig](BotProtectionConfig.md)
`knownDevicesMaxDays` | number
`notifyOnNewDevice` | boolean

## Example

```typescript
import type { ThreatConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "botProtection": null,
  "knownDevicesMaxDays": null,
  "notifyOnNewDevice": null,
} satisfies ThreatConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ThreatConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


