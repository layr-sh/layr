interface DataGridState {
  columns: string[];
  offset: number;
  orderColumn: string;
  direction: string;
  filterColumn: string;
  filterOperator: string;
  filterValue: string;
}

export default function () {
  const grids = useState<Record<string, DataGridState>>("data-workspace-grids", () => ({}));
  const handoff = useState<{ table: string; params: Record<string, string> } | null>(
    "data-workspace-request",
    () => null,
  );
  const auth = useAuth();
  const owner = useState<string | undefined>("data-workspace-owner", () => undefined);
  watch(
    () => auth.state.value?.user.id,
    (id) => {
      if (owner.value === id) return;
      grids.value = {};
      handoff.value = null;
      owner.value = id;
    },
    { immediate: true },
  );
  const toast = useToast();

  async function copy(text: string, title = "Copied to clipboard") {
    try {
      await navigator.clipboard.writeText(text);
      toast.add({ title, color: "success" });
    } catch {
      toast.add({
        title: "Clipboard unavailable",
        description: "Select and copy the text manually.",
        color: "error",
      });
    }
  }

  return { grids, handoff, copy };
}
