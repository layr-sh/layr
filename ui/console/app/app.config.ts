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
        overlay: "bg-default!",
        content: "ring-0! shadow-none! data-[state=open]:animate-[fade-in_200ms_var(--ease-out)]! data-[state=closed]:animate-[fade-out_200ms_var(--ease-out)]!",
        header: "px-0 sm:px-0",
        body: "px-0 sm:px-0",
        footer: "px-0 sm:px-0",
        close: "top-3 right-0 bg-elevated/50 p-1 text-muted hover:text-default",
      },
    },
    pinInput: {
      defaultVariants: {
        variant: "subtle",
      },
    },
  },
});
