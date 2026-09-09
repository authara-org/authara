import { minimalSetup } from "codemirror";
import { html } from "@codemirror/lang-html";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { StateEffect, StateField } from "@codemirror/state";
import {
  Decoration,
  DecorationSet,
  EditorView,
  MatchDecorator,
  ViewPlugin,
  ViewUpdate,
  highlightActiveLine,
  highlightActiveLineGutter,
  hoverTooltip,
  lineNumbers,
} from "@codemirror/view";
import { tags } from "@lezer/highlight";

type EditorKind = "html" | "text";
type PreviewStatus = "ready" | "rendering" | "error";
type EditorLineDiagnostic = {
  line: number;
  message: string;
};
type TemplateDiagnostic = EditorLineDiagnostic & {
  part: EditorKind | "subject";
};

const emailTemplatePersistedEventName = "email-template-persisted";
const editorViews = new WeakMap<HTMLTextAreaElement, EditorView>();
const setEditorLineDiagnostic =
  StateEffect.define<EditorLineDiagnostic | null>();

const editorLineDiagnosticField =
  StateField.define<EditorLineDiagnostic | null>({
    create: () => null,
    update: (diagnostic, transaction) => {
      if (transaction.docChanged) diagnostic = null;
      for (const effect of transaction.effects) {
        if (!effect.is(setEditorLineDiagnostic)) continue;
        diagnostic = effect.value;
      }
      return diagnostic;
    },
  });

const editorLineDiagnosticDecorations = EditorView.decorations.compute(
  [editorLineDiagnosticField],
  (state) => {
    const diagnostic = state.field(editorLineDiagnosticField);
    if (!diagnostic) return Decoration.none;
    const lineNumber = Math.min(Math.max(diagnostic.line, 1), state.doc.lines);
    return Decoration.set([
      Decoration.line({
        attributes: { class: "cm-template-error-line" },
      }).range(state.doc.line(lineNumber).from),
    ]);
  },
);

const editorLineDiagnosticTooltip = hoverTooltip((view, position) => {
  const diagnostic = view.state.field(editorLineDiagnosticField);
  if (!diagnostic) return null;

  const lineNumber = Math.min(
    Math.max(diagnostic.line, 1),
    view.state.doc.lines,
  );
  const line = view.state.doc.line(lineNumber);
  if (position < line.from || position > line.to) return null;

  return {
    pos: line.from,
    end: line.to,
    above: true,
    create: () => {
      const dom = document.createElement("div");
      dom.className = "cm-template-error-tooltip";
      dom.textContent = diagnostic.message;
      return { dom };
    },
  };
});

class EditorOverviewRuler {
  private readonly view: EditorView;
  private readonly dom: HTMLDivElement;
  private readonly canvas: HTMLCanvasElement;
  private readonly viewport: HTMLDivElement;
  private readonly errorMarker: HTMLDivElement;
  private readonly resizeObserver: ResizeObserver;
  private readonly themeObserver: MutationObserver;
  private animationFrame = 0;
  private dragging = false;

  constructor(view: EditorView) {
    this.view = view;
    this.dom = document.createElement("div");
    this.dom.className = "cm-overview-ruler";
    this.dom.tabIndex = 0;
    this.dom.setAttribute("role", "scrollbar");
    this.dom.setAttribute("aria-label", "Document overview");
    this.dom.setAttribute("aria-orientation", "vertical");
    this.dom.title = "Document overview — click or drag to scroll";

    this.canvas = document.createElement("canvas");
    this.canvas.className = "cm-overview-canvas";
    this.canvas.setAttribute("aria-hidden", "true");

    this.viewport = document.createElement("div");
    this.viewport.className = "cm-overview-viewport";
    this.viewport.setAttribute("aria-hidden", "true");

    this.errorMarker = document.createElement("div");
    this.errorMarker.className = "cm-overview-error";
    this.errorMarker.hidden = true;

    this.dom.append(this.canvas, this.viewport, this.errorMarker);
    view.dom.appendChild(this.dom);

    this.dom.addEventListener("pointerdown", this.handlePointerDown);
    this.dom.addEventListener("pointermove", this.handlePointerMove);
    this.dom.addEventListener("pointerup", this.handlePointerUp);
    this.dom.addEventListener("pointercancel", this.handlePointerUp);
    this.dom.addEventListener("keydown", this.handleKeyDown);
    view.scrollDOM.addEventListener("scroll", this.scheduleRender, {
      passive: true,
    });

    this.resizeObserver = new ResizeObserver(this.scheduleRender);
    this.resizeObserver.observe(view.dom);
    this.themeObserver = new MutationObserver(this.scheduleRender);
    this.themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });
    this.scheduleRender();
  }

  update(update: ViewUpdate) {
    const diagnosticChanged =
      update.startState.field(editorLineDiagnosticField) !==
      update.state.field(editorLineDiagnosticField);
    if (
      update.docChanged ||
      update.selectionSet ||
      update.viewportChanged ||
      update.geometryChanged ||
      diagnosticChanged
    ) {
      this.scheduleRender();
    }
  }

  destroy() {
    if (this.animationFrame) cancelAnimationFrame(this.animationFrame);
    this.resizeObserver.disconnect();
    this.themeObserver.disconnect();
    this.view.scrollDOM.removeEventListener("scroll", this.scheduleRender);
    this.dom.removeEventListener("pointerdown", this.handlePointerDown);
    this.dom.removeEventListener("pointermove", this.handlePointerMove);
    this.dom.removeEventListener("pointerup", this.handlePointerUp);
    this.dom.removeEventListener("pointercancel", this.handlePointerUp);
    this.dom.removeEventListener("keydown", this.handleKeyDown);
    this.dom.remove();
  }

  private readonly scheduleRender = () => {
    if (this.animationFrame) return;
    this.animationFrame = requestAnimationFrame(() => {
      this.animationFrame = 0;
      this.render();
    });
  };

  private render() {
    const width = this.dom.clientWidth;
    const height = this.dom.clientHeight;
    if (!width || !height) return;

    const pixelRatio = window.devicePixelRatio || 1;
    const canvasWidth = Math.round(width * pixelRatio);
    const canvasHeight = Math.round(height * pixelRatio);
    if (
      this.canvas.width !== canvasWidth ||
      this.canvas.height !== canvasHeight
    ) {
      this.canvas.width = canvasWidth;
      this.canvas.height = canvasHeight;
    }

    const context = this.canvas.getContext("2d");
    if (!context) return;
    context.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);
    context.clearRect(0, 0, width, height);

    const styles = getComputedStyle(this.dom);
    const document = this.view.state.doc;
    const totalLines = document.lines;
    const maximumSamples = Math.max(1, Math.floor(height * 2));
    const sampleStep = Math.max(1, Math.ceil(totalLines / maximumSamples));
    const lineColor = styles.getPropertyValue("--cm-overview-line").trim();
    const placeholderColor = styles
      .getPropertyValue("--cm-overview-placeholder")
      .trim();

    for (
      let lineNumber = 1;
      lineNumber <= totalLines;
      lineNumber += sampleStep
    ) {
      const text = document.line(lineNumber).text.trim();
      if (!text) continue;
      const y = ((lineNumber - 0.5) / totalLines) * height;
      const markerWidth = Math.min(width - 4, 3 + Math.sqrt(text.length));
      const isPlaceholder = text.includes("{{");
      const markerHeight = isPlaceholder ? 3 : 1;
      context.fillStyle = isPlaceholder ? placeholderColor : lineColor;
      context.fillRect(
        width - markerWidth - 2,
        Math.floor(y - markerHeight / 2),
        markerWidth,
        markerHeight,
      );
    }

    const activeLine = document.lineAt(
      this.view.state.selection.main.head,
    ).number;
    context.fillStyle = styles.getPropertyValue("--cm-overview-active").trim();
    context.fillRect(
      1,
      Math.floor(((activeLine - 0.5) / totalLines) * height),
      width - 2,
      2,
    );

    const { scrollTop, scrollHeight, clientHeight } = this.view.scrollDOM;
    const maximumScroll = Math.max(0, scrollHeight - clientHeight);
    const viewportHeight =
      maximumScroll === 0
        ? height
        : Math.max(
            20,
            Math.min(height, (clientHeight / scrollHeight) * height),
          );
    const viewportTop =
      maximumScroll === 0
        ? 0
        : (scrollTop / maximumScroll) * (height - viewportHeight);
    this.viewport.style.top = `${viewportTop}px`;
    this.viewport.style.height = `${viewportHeight}px`;

    const diagnostic = this.view.state.field(editorLineDiagnosticField);
    if (diagnostic) {
      const diagnosticLine = Math.min(Math.max(diagnostic.line, 1), totalLines);
      this.errorMarker.hidden = false;
      this.errorMarker.style.top = `${((diagnosticLine - 0.5) / totalLines) * height}px`;
      this.errorMarker.title = diagnostic.message;
    } else {
      this.errorMarker.hidden = true;
      this.errorMarker.removeAttribute("title");
    }

    const firstVisibleLine = document.lineAt(this.view.viewport.from).number;
    this.dom.setAttribute("aria-valuemin", "1");
    this.dom.setAttribute("aria-valuemax", String(totalLines));
    this.dom.setAttribute("aria-valuenow", String(firstVisibleLine));
    this.dom.setAttribute(
      "aria-valuetext",
      `Line ${firstVisibleLine} of ${totalLines}`,
    );
  }

  private scrollToPointer(clientY: number) {
    const bounds = this.dom.getBoundingClientRect();
    if (!bounds.height) return;
    const ratio = Math.min(
      1,
      Math.max(0, (clientY - bounds.top) / bounds.height),
    );
    const { scrollHeight, clientHeight } = this.view.scrollDOM;
    this.view.scrollDOM.scrollTop = Math.min(
      Math.max(0, ratio * scrollHeight - clientHeight / 2),
      Math.max(0, scrollHeight - clientHeight),
    );
  }

  private readonly handlePointerDown = (event: PointerEvent) => {
    if (event.button !== 0) return;
    event.preventDefault();
    this.dragging = true;
    this.dom.setPointerCapture(event.pointerId);
    this.view.focus();
    this.scrollToPointer(event.clientY);
  };

  private readonly handlePointerMove = (event: PointerEvent) => {
    if (!this.dragging) return;
    event.preventDefault();
    this.scrollToPointer(event.clientY);
  };

  private readonly handlePointerUp = (event: PointerEvent) => {
    if (!this.dragging) return;
    this.dragging = false;
    if (this.dom.hasPointerCapture(event.pointerId)) {
      this.dom.releasePointerCapture(event.pointerId);
    }
  };

  private readonly handleKeyDown = (event: KeyboardEvent) => {
    const scrollDOM = this.view.scrollDOM;
    const lineHeight = this.view.defaultLineHeight;
    let nextScrollTop: number | null = null;
    switch (event.key) {
      case "ArrowUp":
        nextScrollTop = scrollDOM.scrollTop - lineHeight;
        break;
      case "ArrowDown":
        nextScrollTop = scrollDOM.scrollTop + lineHeight;
        break;
      case "PageUp":
        nextScrollTop = scrollDOM.scrollTop - scrollDOM.clientHeight;
        break;
      case "PageDown":
        nextScrollTop = scrollDOM.scrollTop + scrollDOM.clientHeight;
        break;
      case "Home":
        nextScrollTop = 0;
        break;
      case "End":
        nextScrollTop = scrollDOM.scrollHeight;
        break;
    }
    if (nextScrollTop === null) return;
    event.preventDefault();
    scrollDOM.scrollTop = nextScrollTop;
  };
}

const editorOverviewRuler = ViewPlugin.fromClass(EditorOverviewRuler);

const placeholderMatcher = new MatchDecorator({
  regexp: /\{\{\s*[a-zA-Z_][a-zA-Z0-9_]*\s*\}\}/g,
  decoration: Decoration.mark({ class: "cm-template-placeholder" }),
});

const placeholderHighlighting = ViewPlugin.fromClass(
  class {
    placeholders: DecorationSet;

    constructor(view: EditorView) {
      this.placeholders = placeholderMatcher.createDeco(view);
    }

    update(update: ViewUpdate) {
      this.placeholders = placeholderMatcher.updateDeco(
        update,
        this.placeholders,
      );
    }
  },
  {
    decorations: (instance) => instance.placeholders,
  },
);

const readableSyntaxHighlighting = HighlightStyle.define([
  {
    tag: [tags.tagName, tags.typeName, tags.className],
    color: "var(--cm-tag)",
    fontWeight: "600",
  },
  {
    tag: [tags.attributeName, tags.propertyName],
    color: "var(--cm-attribute)",
  },
  {
    tag: [tags.string, tags.attributeValue, tags.url],
    color: "var(--cm-string)",
  },
  {
    tag: [tags.number, tags.bool, tags.atom, tags.null],
    color: "var(--cm-literal)",
  },
  {
    tag: [tags.keyword, tags.modifier, tags.operatorKeyword],
    color: "var(--cm-keyword)",
    fontWeight: "600",
  },
  {
    tag: [tags.angleBracket, tags.punctuation, tags.operator],
    color: "var(--cm-punctuation)",
  },
  {
    tag: [tags.comment, tags.documentMeta, tags.meta],
    color: "var(--cm-comment)",
    fontStyle: "italic",
  },
  {
    tag: tags.invalid,
    color: "var(--cm-invalid)",
    textDecoration: "underline wavy",
  },
]);

const editorTheme = EditorView.theme({
  "&": {
    "--cm-text": "#1f2937",
    "--cm-tag": "#047857",
    "--cm-attribute": "#1d4ed8",
    "--cm-string": "#a16207",
    "--cm-literal": "#be123c",
    "--cm-keyword": "#6d28d9",
    "--cm-punctuation": "#475569",
    "--cm-comment": "#64748b",
    "--cm-invalid": "#b91c1c",
    "--cm-placeholder": "#1d4ed8",
    "--cm-placeholder-bg": "rgb(37 99 235 / 0.13)",
    "--cm-selection": "rgb(59 130 246 / 0.2)",
    "--cm-error-line-bg": "rgb(254 226 226 / 0.72)",
    "--cm-error-line-edge": "#dc2626",
    "--cm-overview-track": "rgb(248 250 252 / 0.96)",
    "--cm-overview-line": "rgb(100 116 139 / 0.65)",
    "--cm-overview-placeholder": "rgb(37 99 235 / 0.9)",
    "--cm-overview-active": "#16a34a",
    "--cm-overview-viewport": "rgb(100 116 139 / 0.2)",
    "--cm-overview-viewport-edge": "rgb(100 116 139 / 0.45)",
    height: "100%",
    position: "relative",
    backgroundColor: "transparent",
    color: "var(--cm-text)",
    caretColor: "#2563eb",
    fontSize: "0.875rem",
  },
  ".dark &": {
    "--cm-text": "#e2e8f0",
    "--cm-tag": "#6ee7b7",
    "--cm-attribute": "#93c5fd",
    "--cm-string": "#fcd34d",
    "--cm-literal": "#fda4af",
    "--cm-keyword": "#c4b5fd",
    "--cm-punctuation": "#cbd5e1",
    "--cm-comment": "#94a3b8",
    "--cm-invalid": "#fca5a5",
    "--cm-placeholder": "#bfdbfe",
    "--cm-placeholder-bg": "rgb(96 165 250 / 0.2)",
    "--cm-selection": "rgb(96 165 250 / 0.25)",
    "--cm-error-line-bg": "rgb(127 29 29 / 0.38)",
    "--cm-error-line-edge": "#f87171",
    "--cm-overview-track": "rgb(15 23 42 / 0.96)",
    "--cm-overview-line": "rgb(148 163 184 / 0.6)",
    "--cm-overview-placeholder": "rgb(96 165 250 / 0.95)",
    "--cm-overview-active": "#4ade80",
    "--cm-overview-viewport": "rgb(148 163 184 / 0.2)",
    "--cm-overview-viewport-edge": "rgb(148 163 184 / 0.45)",
    caretColor: "#60a5fa",
  },
  "&.cm-focused": {
    outline: "none",
  },
  ".cm-scroller": {
    fontFamily:
      'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace',
    lineHeight: "1.625rem",
    overflow: "auto",
    paddingRight: "1rem",
  },
  ".cm-content": {
    padding: "1rem 0",
  },
  ".cm-line": {
    padding: "0 1rem",
  },
  ".cm-gutters": {
    backgroundColor: "transparent",
    borderRight: "1px solid rgb(156 163 175 / 0.25)",
    color: "rgb(107 114 128)",
  },
  ".cm-activeLine, .cm-activeLineGutter": {
    backgroundColor: "rgb(59 130 246 / 0.09)",
  },
  ".cm-selectionBackground, &.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground":
    {
      backgroundColor: "var(--cm-selection)",
    },
  ".cm-cursor, .cm-dropCursor": {
    borderLeftColor: "currentColor",
  },
  ".cm-template-placeholder": {
    borderRadius: "0.25rem",
    backgroundColor: "var(--cm-placeholder-bg)",
    color: "var(--cm-placeholder)",
    fontWeight: "600",
  },
  ".cm-template-error-line": {
    backgroundColor: "var(--cm-error-line-bg)",
    boxShadow: "inset 3px 0 0 var(--cm-error-line-edge)",
    textDecoration: "underline wavy var(--cm-error-line-edge)",
    textUnderlineOffset: "3px",
  },
  ".cm-overview-ruler": {
    position: "absolute",
    zIndex: "8",
    top: "0",
    right: "0",
    bottom: "0",
    width: "1rem",
    overflow: "hidden",
    borderLeft: "1px solid rgb(100 116 139 / 0.22)",
    backgroundColor: "var(--cm-overview-track)",
    cursor: "pointer",
    touchAction: "none",
    userSelect: "none",
  },
  ".cm-overview-ruler:focus-visible": {
    outline: "2px solid var(--cm-placeholder)",
    outlineOffset: "-2px",
  },
  ".cm-overview-canvas": {
    position: "absolute",
    inset: "0",
    width: "100%",
    height: "100%",
    pointerEvents: "none",
  },
  ".cm-overview-viewport": {
    position: "absolute",
    zIndex: "2",
    right: "1px",
    left: "1px",
    minHeight: "1.25rem",
    border: "1px solid var(--cm-overview-viewport-edge)",
    backgroundColor: "var(--cm-overview-viewport)",
    pointerEvents: "none",
  },
  ".cm-overview-error": {
    position: "absolute",
    zIndex: "3",
    right: "1px",
    left: "1px",
    height: "4px",
    transform: "translateY(-50%)",
    backgroundColor: "var(--cm-error-line-edge)",
    boxShadow: "0 0 0 1px var(--cm-overview-track)",
  },
  ".cm-tooltip.cm-tooltip-hover": {
    border: "0",
    backgroundColor: "transparent",
  },
  ".cm-template-error-tooltip": {
    maxWidth: "min(36rem, calc(100vw - 2rem))",
    border: "1px solid var(--cm-error-line-edge)",
    borderRadius: "0.5rem",
    backgroundColor: "#fff7f7",
    color: "#991b1b",
    padding: "0.625rem 0.75rem",
    fontFamily:
      'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace',
    fontSize: "0.75rem",
    lineHeight: "1.25rem",
    whiteSpace: "pre-wrap",
    boxShadow: "0 8px 24px rgb(15 23 42 / 0.18)",
  },
  ".dark & .cm-template-error-tooltip": {
    backgroundColor: "#450a0a",
    color: "#fecaca",
  },
});

function mountEditor(textarea: HTMLTextAreaElement, kind: EditorKind) {
  if (textarea.dataset.editorInitialized === "true") return;

  const editorID = textarea.id;
  const label = textarea.labels?.item(0);
  if (label && !label.id) label.id = `${editorID}-label`;
  const fillAvailableHeight =
    textarea.dataset.emailTemplateEditorFill === "true";

  const mount = document.createElement("div");
  mount.className = [
    ...(fillAvailableHeight
      ? ["min-h-0", "flex-1"]
      : ["mt-2", kind === "html" ? "h-[32rem]" : "h-64"]),
    "overflow-hidden",
    "rounded-xl",
    "border",
    "border-grey-200",
    "bg-white",
    "text-grey-900",
    "focus-within:ring-2",
    "focus-within:ring-blue-200",
    "dark:border-grey-800",
    "dark:bg-grey-950",
    "dark:text-grey-100",
    "dark:focus-within:ring-blue-900/40",
  ].join(" ");

  textarea.insertAdjacentElement("afterend", mount);
  textarea.hidden = true;
  textarea.dataset.editorInitialized = "true";

  const editor = new EditorView({
    doc: textarea.value,
    parent: mount,
    extensions: [
      minimalSetup,
      lineNumbers(),
      highlightActiveLine(),
      highlightActiveLineGutter(),
      ...(kind === "text" ? [EditorView.lineWrapping] : []),
      ...(kind === "html" ? [html()] : []),
      placeholderHighlighting,
      editorLineDiagnosticField,
      editorLineDiagnosticDecorations,
      editorLineDiagnosticTooltip,
      editorOverviewRuler,
      syntaxHighlighting(readableSyntaxHighlighting),
      editorTheme,
      EditorView.contentAttributes.of({
        "aria-label": kind === "html" ? "HTML template" : "Plain-text template",
        ...(label ? { "aria-labelledby": label.id } : {}),
      }),
      EditorView.updateListener.of((update) => {
        if (!update.docChanged) return;
        textarea.value = update.state.doc.toString();
        textarea.dispatchEvent(new Event("input", { bubbles: true }));
      }),
    ],
  });
  editorViews.set(textarea, editor);

  label?.addEventListener("click", (event) => {
    event.preventDefault();
    editor.focus();
  });
}

function destroyEmailTemplateEditors(root: Element) {
  const textareas = [
    ...(root.matches("textarea[data-email-template-editor]")
      ? [root as HTMLTextAreaElement]
      : []),
    ...root.querySelectorAll<HTMLTextAreaElement>(
      "textarea[data-email-template-editor]",
    ),
  ];
  textareas.forEach((textarea) => {
    const editor = editorViews.get(textarea);
    if (!editor) return;
    editor.destroy();
    editorViews.delete(textarea);
  });
}

function readTemplateDiagnostic(root: ParentNode): TemplateDiagnostic | null {
  const element = root.querySelector<HTMLElement>(
    "[data-email-template-diagnostic]",
  );
  if (!element) return null;

  const part = element.dataset.templatePart;
  const line = Number.parseInt(element.dataset.templateLine ?? "", 10);
  const message = element.dataset.templateMessage;
  if (
    (part !== "subject" && part !== "text" && part !== "html") ||
    !Number.isInteger(line) ||
    !message
  ) {
    return null;
  }
  return { part, line, message };
}

function applyTemplateDiagnostic(root: ParentNode) {
  const diagnostic = readTemplateDiagnostic(root);
  const workspace = root.querySelector<HTMLElement>(
    "[data-email-template-workspace]",
  );
  if (!workspace) return;

  workspace
    .querySelectorAll<HTMLTextAreaElement>(
      "textarea[data-email-template-editor]",
    )
    .forEach((textarea) => {
      const editor = editorViews.get(textarea);
      if (!editor) return;
      const kind = textarea.dataset.emailTemplateEditor;
      const lineDiagnostic =
        diagnostic && diagnostic.part === kind && diagnostic.line > 0
          ? { line: diagnostic.line, message: diagnostic.message }
          : null;
      editor.dispatch({ effects: setEditorLineDiagnostic.of(lineDiagnostic) });
    });

  const subject = workspace.querySelector<HTMLInputElement>(
    "#email-template-subject",
  );
  if (subject) {
    const hasSubjectError = diagnostic?.part === "subject";
    subject.classList.toggle("!border-red-500", hasSubjectError);
    if (hasSubjectError) {
      subject.setAttribute("aria-invalid", "true");
      subject.title = diagnostic.message;
    } else {
      subject.removeAttribute("aria-invalid");
      subject.removeAttribute("title");
    }
  }

  const signature = diagnostic
    ? `${diagnostic.part}:${diagnostic.line}:${diagnostic.message}`
    : "";
  if (
    diagnostic &&
    diagnostic.line > 0 &&
    diagnostic.part !== "subject" &&
    workspace.dataset.activeDiagnostic !== signature
  ) {
    workspace.dispatchEvent(
      new CustomEvent("email-template-diagnostic", {
        bubbles: true,
        detail: { part: diagnostic.part },
      }),
    );
  }
  workspace.dataset.activeDiagnostic = signature;
}

function setPreviewStatus(
  status: HTMLElement,
  state: PreviewStatus,
  message: string,
) {
  status.textContent = message;
  status.classList.remove(
    "text-green-700",
    "dark:text-green-300",
    "text-blue-700",
    "dark:text-blue-300",
    "text-red-700",
    "dark:text-red-300",
  );
  status.classList.add(
    ...(state === "ready"
      ? ["text-green-700", "dark:text-green-300"]
      : state === "rendering"
        ? ["text-blue-700", "dark:text-blue-300"]
        : ["text-red-700", "dark:text-red-300"]),
  );
}

function initPreviewStatus(root: ParentNode) {
  const workspace = root.querySelector<HTMLElement>(
    "[data-email-template-workspace]",
  );
  if (!workspace || workspace.dataset.previewStatusInitialized === "true") {
    return;
  }

  const status = workspace.querySelector<HTMLElement>(
    "[data-email-template-preview-status]",
  );
  if (!status) return;

  workspace.dataset.previewStatusInitialized = "true";
  workspace.addEventListener(emailTemplatePersistedEventName, () => {
    setPreviewStatus(status, "ready", "Preview rendered");
  });
  const isPreviewRequest = (event: Event) => {
    const requestElement = (event as CustomEvent<{ elt?: Element }>).detail
      ?.elt;
    return requestElement?.hasAttribute("data-email-template-preview-request");
  };

  workspace.addEventListener("htmx:beforeRequest", (event) => {
    if (!isPreviewRequest(event)) return;
    setPreviewStatus(status, "rendering", "Rendering preview…");
  });

  workspace.addEventListener("htmx:afterRequest", (event) => {
    if (!isPreviewRequest(event)) return;

    const xhr = (event as CustomEvent<{ xhr?: XMLHttpRequest }>).detail?.xhr;
    if (xhr?.status === 200) {
      setPreviewStatus(status, "ready", "Preview rendered");
      return;
    }
    if (xhr?.status === 422) {
      setPreviewStatus(status, "error", "Template has errors");
      return;
    }

    const suffix = xhr?.status ? ` (${xhr.status})` : "";
    setPreviewStatus(status, "error", `Preview request failed${suffix}`);
  });
}

export function initEmailTemplateEditors(root: ParentNode) {
  initPreviewStatus(root);
  root
    .querySelectorAll<HTMLTextAreaElement>(
      "textarea[data-email-template-editor]",
    )
    .forEach((textarea) => {
      const kind = textarea.dataset.emailTemplateEditor;
      if (kind === "html" || kind === "text") {
        mountEditor(textarea, kind);
      }
    });
  applyTemplateDiagnostic(root);
}

document.body.addEventListener("htmx:beforeCleanupElement", (event) => {
  const root = (event as CustomEvent<{ elt?: Element }>).detail?.elt;
  if (root) destroyEmailTemplateEditors(root);
});
