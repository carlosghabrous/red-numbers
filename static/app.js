// Small, unobtrusive behaviors for the dashboard. Kept out of inline event
// handler attributes (onchange=, onsubmit=) because the app's
// Content-Security-Policy intentionally has no 'unsafe-inline' for scripts.
document.addEventListener('DOMContentLoaded', function () {
	// Selecting a date-range preset (All/Current/Previous Month) applies the
	// filter immediately instead of requiring a separate "Apply filters" click.
	document.querySelectorAll('.js-auto-apply').forEach(function (input) {
		input.addEventListener('change', function () {
			if (input.form) {
				input.form.submit();
			}
		});
	});

	// Confirm before deleting every stored expense and classification log.
	document.querySelectorAll('.js-confirm-delete-all').forEach(function (form) {
		form.addEventListener('submit', function (event) {
			if (!window.confirm('Delete all expenses and classification logs?')) {
				event.preventDefault();
			}
		});
	});
});
