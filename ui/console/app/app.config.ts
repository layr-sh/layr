export default defineAppConfig({
  ui: {
    alert: {
      defaultVariants: {
        variant: "subtle",
      },
    },
    colors: {
      primary: "teal",
      neutral: "neutral",
    },
    empty: {
      defaultVariants: {
        variant: "naked",
      },
    },
    input: {
      defaultVariants: {
        variant: "subtle",
      },
    },
    select: {
      defaultVariants: {
        variant: "subtle",
      },
    },
    textarea: {
      defaultVariants: {
        variant: "subtle",
      },
    },
    selectMenu: {
      defaultVariants: {
        variant: "subtle",
      },
    },
    inputMenu: {
      defaultVariants: {
        variant: "subtle",
      },
    },
    inputNumber: {
      defaultVariants: {
        variant: "subtle",
      },
    },
    inputTags: {
      defaultVariants: {
        variant: "subtle",
      },
    },
    inputDate: {
      defaultVariants: {
        variant: "subtle",
      },
    },
    inputTime: {
      defaultVariants: {
        variant: "subtle",
      },
    },
    modal: {
      slots: {
        overlay:
          "bg-default! data-[state=open]:animate-[fade-in_200ms_var(--ease-in-out)]! data-[state=closed]:animate-[fade-out_600ms_var(--ease-in-out)]!",
        content:
          "ring-0! shadow-none! data-[state=open]:animate-[modal-slide-up_300ms_var(--ease-in-out)]! data-[state=closed]:animate-[modal-slide-down_300ms_var(--ease-in-out)]!",
        header: "px-0 sm:px-0",
        body: "px-0 sm:px-0",
        footer: "px-0 sm:px-0",
        close:
          "top-4 right-0 bg-elevated hover:bg-elevated/75 p-0.5 text-muted hover:text-default [&>span]:size-4!",
      },
    },
    pinInput: {
      defaultVariants: {
        variant: "subtle",
      },
    },
    table: {
      slots: {
        th: "first:pl-0 last:pr-0 whitespace-nowrap",
        td: "first:pl-0 last:pr-0",
      },
    },
  },
});
