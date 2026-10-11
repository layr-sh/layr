
# GetInfoResponse

GetInfoResponse schema

## Properties

Name | Type
------------ | -------------
`alpha` | boolean
`bands` | number
`colorspace` | string
`exif` | { [key: string]: any | null; }
`format` | string
`height` | number
`mimeType` | string
`orientation` | number
`pages` | number
`size` | number
`videoMeta` | { [key: string]: any | null; }
`width` | number
`xmp` | string

## Example

```typescript
import type { GetInfoResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "alpha": null,
  "bands": null,
  "colorspace": null,
  "exif": null,
  "format": null,
  "height": null,
  "mimeType": null,
  "orientation": null,
  "pages": null,
  "size": null,
  "videoMeta": null,
  "width": null,
  "xmp": null,
} satisfies GetInfoResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as GetInfoResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


