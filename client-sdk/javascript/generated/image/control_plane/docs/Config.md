
# Config

Config schema

## Properties

Name | Type
------------ | -------------
`allowInsecure` | boolean
`allowedDomains` | Array&lt;string&gt;
`cacheTtlSeconds` | number
`defaultQuality` | number
`jpegProgressive` | boolean
`keepCopyright` | boolean
`maxAnimationFrames` | number
`maxSrcResolution` | number
`pngInterlaced` | boolean
`stripColorProfile` | boolean
`stripMetadata` | boolean
`watermarkOpacity` | number
`watermarkUrl` | string

## Example

```typescript
import type { Config } from ''

// TODO: Update the object below with actual values
const example = {
  "allowInsecure": null,
  "allowedDomains": null,
  "cacheTtlSeconds": null,
  "defaultQuality": null,
  "jpegProgressive": null,
  "keepCopyright": null,
  "maxAnimationFrames": null,
  "maxSrcResolution": null,
  "pngInterlaced": null,
  "stripColorProfile": null,
  "stripMetadata": null,
  "watermarkOpacity": null,
  "watermarkUrl": null,
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


