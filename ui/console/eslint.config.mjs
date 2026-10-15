// eslint still needed because oxlint doesn't work with vue sfc template yet
// see https://github.com/oxc-project/oxc/issues/15761
import withNuxt from "./.nuxt/eslint.config.mjs";
import oxlint from "eslint-plugin-oxlint";

export default withNuxt(...oxlint.configs["flat/recommended"], {
  // Resolve formatting conflict between oxfmt (Prettier-compatible) and eslint-plugin-vue:
  // oxfmt formats void HTML elements as self-closing (<input />, <img />), whereas default
  // vue/html-self-closing expects void: "never" (<input>). Setting void: "any" allows
  // oxfmt to manage void element self-closing without ESLint warnings.
  rules: {
    "vue/html-self-closing": [
      "error",
      {
        html: {
          void: "any",
          normal: "always",
          component: "always",
        },
        svg: "always",
        math: "always",
      },
    ],
  },
});
