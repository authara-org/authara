export function initVerificationCodeForm(root: ParentNode = document): void {
  const form = root.querySelector('#verification-code-input-form') as HTMLFormElement | null;
  if (!form) return;

  if (form.dataset.bound === 'true') return;
  form.dataset.bound = 'true';

  const hiddenInput = form.querySelector('#hidden-code-input') as HTMLInputElement | null;
  const inputs = Array.from(
    form.querySelectorAll('.code-input'),
  ) as HTMLInputElement[];

  if (!hiddenInput || inputs.length === 0) return;

  const syncHidden = () => {
    hiddenInput.value = inputs.map(input => input.value).join('');
  };

  const focusInput = (index: number) => {
    if (index < 0 || index >= inputs.length) return;
    inputs[index].focus();
    inputs[index].select();
  };

  form.addEventListener(
    'submit',
    () => {
      syncHidden();
    },
    true,
  );

  inputs.forEach((input, index) => {
    input.addEventListener('input', () => {
      const value = input.value.replace(/\D/g, '').slice(0, 1);
      input.value = value;
      syncHidden();

      if (value && index < inputs.length - 1) {
        focusInput(index + 1);
      }
    });

    input.addEventListener('keydown', (e: KeyboardEvent) => {
      if (e.key === 'Backspace') {
        if (input.value !== '') {
          input.value = '';
          syncHidden();
          e.preventDefault();
          return;
        }

        if (index > 0) {
          focusInput(index - 1);
          e.preventDefault();
        }
      }

      if (e.key === 'Delete') {
        input.value = '';
        syncHidden();
        e.preventDefault();
      }

      if (e.key === 'ArrowLeft' && index > 0) {
        focusInput(index - 1);
        e.preventDefault();
      }

      if (e.key === 'ArrowRight' && index < inputs.length - 1) {
        focusInput(index + 1);
        e.preventDefault();
      }
    });

    input.addEventListener('paste', (e: ClipboardEvent) => {
      e.preventDefault();

      const pasted = (e.clipboardData?.getData('text') || '')
        .replace(/\D/g, '')
        .slice(0, inputs.length);

      if (!pasted) return;

      for (let i = 0; i < inputs.length; i++) {
        inputs[i].value = pasted[i] || '';
      }

      syncHidden();
      focusInput(Math.min(pasted.length - 1, inputs.length - 1));
    });
  });

  syncHidden();
  focusInput(0);
}
