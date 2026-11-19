import type { FormError } from "@nuxt/ui";
import type { FunctionControlPlaneModels } from "layr-client";

export function validFunctionFilePath(path: string) {
  return (
    !!path &&
    !path.includes("\\") &&
    !path.startsWith("/") &&
    path.split("/").every((part) => part !== "" && part !== "." && part !== "..")
  );
}
export function validFunctionDomain(domain: string) {
  return (
    domain.length <= 253 &&
    domain.includes(".") &&
    domain.split(".").every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(label))
  );
}
export function validFunctionDate(date: string) {
  const value = new Date(date);
  return (
    /^\d{4}-\d{2}-\d{2}$/.test(date) &&
    Number.isFinite(value.getTime()) &&
    value.toISOString().slice(0, 10) === date
  );
}
export interface FunctionArchive {
  name: string;
  size: number;
  format: "zip" | "tar";
  content: string;
}
export async function readFunctionArchive(file: File, maxBytes?: number): Promise<FunctionArchive> {
  const extension = file.name.toLowerCase().split(".").pop();
  if (extension !== "zip" && extension !== "tar")
    throw new Error("Choose a ZIP (.zip) or uncompressed TAR (.tar) archive.");
  if (!file.size)
    throw new Error("The archive is empty. Choose a bundle containing your entrypoint.");
  if (maxBytes && file.size > maxBytes)
    throw new Error(`The archive exceeds the service limit of ${maxBytes.toLocaleString()} bytes.`);
  const uint8Array = new Uint8Array(await file.arrayBuffer());
  let binary = "";
  for (let offset = 0; offset < uint8Array.length; offset += 32768)
    binary += String.fromCharCode(...uint8Array.subarray(offset, offset + 32768));
  return { name: file.name, size: file.size, format: extension, content: btoa(binary) };
}
export interface FunctionDeploymentForm {
  bundleSource?: "archive" | "files";
  archive?: FunctionArchive | null;
  files: { path: string; content: string }[];
  variables: { name: string; value: string }[];
  overrideRuntime: boolean;
  compatibilityDate: string;
  compatibilityFlags: string | string[];
  overrideFlags?: boolean;
}
export function resolveFunctionDeployment(form: FunctionDeploymentForm, entrypoint: string) {
  const errors: FormError[] = [];
  const bundleFiles: Record<string, string> = Object.create(null);
  if (form.bundleSource === "archive") {
    if (!form.archive?.content)
      errors.push({
        name: "archive",
        message: "Select a ZIP or TAR bundle and wait for it to finish reading.",
      });
  } else {
    for (const [index, file] of form.files.entries()) {
      const path = file.path.trim();
      if (!validFunctionFilePath(path) || path in bundleFiles)
        errors.push({
          name: `files.${index}.path`,
          message: "Use a unique relative file path without . or .. segments.",
        });
      else bundleFiles[path] = file.content;
    }
    if (!(entrypoint in bundleFiles) || !bundleFiles[entrypoint]?.trim())
      errors.push({
        name: "files",
        message: `Include a non-empty entrypoint file named ${entrypoint}.`,
      });
    if (
      errors.some((error) => error.name!.startsWith("files.")) &&
      !errors.some((error) => error.name === "files")
    )
      errors.push({ name: "files", message: "Correct the invalid file paths before publishing." });
  }
  const environmentVariables: Record<string, string> = Object.create(null);
  for (const [index, variable] of form.variables.entries()) {
    const name = variable.name.trim();
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(name) || name in environmentVariables)
      errors.push({
        name: `variables.${index}.name`,
        message:
          "Use a unique environment key containing letters, numbers, and underscores, not starting with a number.",
      });
    else environmentVariables[name] = variable.value;
  }
  const input: FunctionControlPlaneModels.CreateDeploymentInput = { environmentVariables };
  if (form.bundleSource === "archive") {
    input.bundleFormat = form.archive?.format;
    input.bundleContent = form.archive?.content;
  } else input.bundleFiles = bundleFiles;
  if (form.overrideRuntime) {
    if (!validFunctionDate(form.compatibilityDate))
      errors.push({ name: "compatibilityDate", message: "Choose a valid compatibility date." });
    input.workerdRuntimeConfig = {
      compatibilityDate: form.compatibilityDate,
    };
    if (form.overrideFlags !== false) {
      input.workerdRuntimeConfig.compatibilityFlags = [
        ...new Set(
          (Array.isArray(form.compatibilityFlags)
            ? form.compatibilityFlags
            : form.compatibilityFlags.split(/[\n,]/)
          )
            .map((flag) => flag.trim())
            .filter(Boolean),
        ),
      ];
      if (
        input.workerdRuntimeConfig.compatibilityFlags.some(
          (flag) => !/^[a-z][a-z0-9_]*$/.test(flag),
        )
      )
        errors.push({
          name: "compatibilityFlags",
          message: "Enter compatibility flag names without spaces, such as nodejs_compat.",
        });
    }
  }
  return { input, errors };
}
