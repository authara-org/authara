type DropdownChangeDetail = {
  value: string;
  label: string;
};

type AlpineMagic = {
  $el: HTMLElement;
  $nextTick: (callback: () => void) => void;
  $refs: {
    input: HTMLInputElement;
    trigger: HTMLButtonElement;
  };
};

type DropdownState = {
  open: boolean;
  value: string;
  label: string;
  init(this: DropdownContext): void;
  toggle(this: DropdownContext): void;
  openMenu(this: DropdownContext, position?: "selected" | "last"): void;
  close(this: DropdownContext, restoreFocus: boolean): void;
  selectOption(this: DropdownContext, option: HTMLButtonElement): void;
  handleTriggerKeydown(this: DropdownContext, event: KeyboardEvent): void;
  handleOptionKeydown(
    this: DropdownContext,
    event: KeyboardEvent,
    option: HTMLButtonElement,
  ): void;
};

type DropdownContext = DropdownState & AlpineMagic;

declare global {
  interface Window {
    customDropdown: () => DropdownState;
  }
}

function options(root: HTMLElement): HTMLButtonElement[] {
  return Array.from(
    root.querySelectorAll<HTMLButtonElement>("[data-dropdown-option]"),
  );
}

function focusOption(context: DropdownContext, index: number): void {
  const items = options(context.$el);
  if (items.length === 0) return;

  const wrappedIndex = (index + items.length) % items.length;
  items[wrappedIndex].focus();
}

document.addEventListener("alpine:init", () => {
  window.customDropdown = function (): DropdownState {
    let search = "";
    let searchReset: number | undefined;

    function focusByText(context: DropdownContext, character: string): void {
      window.clearTimeout(searchReset);
      search += character.toLocaleLowerCase();
      searchReset = window.setTimeout(() => {
        search = "";
      }, 500);

      const items = options(context.$el);
      const currentIndex = items.indexOf(
        document.activeElement as HTMLButtonElement,
      );
      const ordered = [
        ...items.slice(currentIndex + 1),
        ...items.slice(0, currentIndex + 1),
      ];
      const match = ordered.find((item) =>
        (item.dataset.label ?? "").toLocaleLowerCase().startsWith(search),
      );
      match?.focus();
    }

    return {
      open: false,
      value: "",
      label: "",

      init() {
        this.value = this.$el.dataset.dropdownValue ?? "";
        this.label = this.$el.dataset.dropdownLabel ?? "";
      },

      toggle() {
        if (this.open) {
          this.close(false);
          return;
        }
        this.openMenu();
      },

      openMenu(position = "selected") {
        this.open = true;
        this.$nextTick(() => {
          const items = options(this.$el);
          if (position === "last") {
            focusOption(this, items.length - 1);
            return;
          }
          const selectedIndex = items.findIndex(
            (item) => item.dataset.value === this.value,
          );
          focusOption(this, selectedIndex >= 0 ? selectedIndex : 0);
        });
      },

      close(restoreFocus) {
        if (!this.open) return;
        this.open = false;
        search = "";
        window.clearTimeout(searchReset);
        if (restoreFocus) {
          this.$nextTick(() => this.$refs.trigger.focus());
        }
      },

      selectOption(option) {
        const value = option.dataset.value ?? "";
        const label = option.dataset.label ?? "";
        const change = new CustomEvent<DropdownChangeDetail>(
          "dropdown-change",
          {
            bubbles: true,
            cancelable: true,
            detail: { value, label },
          },
        );

        if (!this.$el.dispatchEvent(change)) {
          this.close(true);
          return;
        }

        this.value = value;
        this.label = label;
        this.$refs.input.value = value;
        this.$refs.input.dispatchEvent(new Event("change", { bubbles: true }));
        this.close(true);
      },

      handleTriggerKeydown(event) {
        switch (event.key) {
          case "ArrowDown":
          case "Enter":
          case " ":
            event.preventDefault();
            this.openMenu();
            break;
          case "ArrowUp":
            event.preventDefault();
            this.openMenu("last");
            break;
          case "Escape":
            event.preventDefault();
            this.close(true);
            break;
          default:
            if (event.key.length === 1 && !event.ctrlKey && !event.metaKey) {
              event.preventDefault();
              this.open = true;
              this.$nextTick(() => focusByText(this, event.key));
            }
        }
      },

      handleOptionKeydown(event, option) {
        const items = options(this.$el);
        const index = items.indexOf(option);
        switch (event.key) {
          case "ArrowDown":
            event.preventDefault();
            focusOption(this, index + 1);
            break;
          case "ArrowUp":
            event.preventDefault();
            focusOption(this, index - 1);
            break;
          case "Home":
            event.preventDefault();
            focusOption(this, 0);
            break;
          case "End":
            event.preventDefault();
            focusOption(this, items.length - 1);
            break;
          case "Enter":
          case " ":
            event.preventDefault();
            this.selectOption(option);
            break;
          case "Escape":
            event.preventDefault();
            this.close(true);
            break;
          case "Tab":
            this.close(false);
            break;
          default:
            if (event.key.length === 1 && !event.ctrlKey && !event.metaKey) {
              focusByText(this, event.key);
            }
        }
      },
    };
  };
});

export {};
