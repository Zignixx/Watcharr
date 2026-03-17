<script lang="ts">
	import { type Media, MediaTypeE } from "@/types";
	import { toUnderstandableStatus } from "../util/helpers";
	import { toShowableRating } from "../rating/helpers";
	import Icon from "../Icon.svelte";
	import Modal from "../Modal.svelte";

	interface Props {
		items: Media[];
		onClose: () => void;
	}

	let { items, onClose }: Props = $props();

	type ExportFormat = "csv" | "json" | "text";
	let format: ExportFormat = $state("csv");

	function typeLabel(media: Media): string {
		switch (media.type) {
			case MediaTypeE.tmdbMovie: return "Movie";
			case MediaTypeE.tmdbShow: return "Show";
			case MediaTypeE.igdbGame: return "Game";
			default: return "";
		}
	}

	function getYear(media: Media): string {
		return media.releaseDate ? new Date(media.releaseDate).getFullYear().toString() : "";
	}

	function buildRows() {
		return items
			.filter(m => m && m.name)
			.map(m => ({
				name: m.name ?? "",
				type: typeLabel(m),
				year: getYear(m),
				status: m.watched?.status
					? toUnderstandableStatus(m.watched.status, m.type === MediaTypeE.igdbGame)
					: "",
				rating: m.watched?.rating ? toShowableRating(m.watched.rating) : "",
				added: m.watched?.createdAt
					? new Date(m.watched.createdAt).toLocaleDateString()
					: "",
				thoughts: m.watched?.thoughts ?? "",
			}));
	}

	function exportCSV() {
		const rows = buildRows();
		const header = "Name,Type,Year,Status,Rating,Added,Thoughts";
		const csvRows = rows.map(r =>
			[r.name, r.type, r.year, r.status, r.rating, r.added, r.thoughts]
				.map(v => `"${String(v).replace(/"/g, '""')}"`)
				.join(",")
		);
		return header + "\n" + csvRows.join("\n");
	}

	function exportJSON() {
		return JSON.stringify(buildRows(), null, 2);
	}

	function exportText() {
		const rows = buildRows();
		const lines = rows.map((r, i) => {
			let line = `${i + 1}. ${r.name}`;
			if (r.year) line += ` (${r.year})`;
			if (r.type) line += ` [${r.type}]`;
			if (r.status) line += ` — ${r.status}`;
			if (r.rating) line += ` — ★ ${r.rating}`;
			return line;
		});
		return lines.join("\n");
	}

	function getExportContent(): string {
		switch (format) {
			case "csv": return exportCSV();
			case "json": return exportJSON();
			case "text": return exportText();
		}
	}

	function getFilename(): string {
		const date = new Date().toISOString().slice(0, 10);
		switch (format) {
			case "csv": return `watcharr-export-${date}.csv`;
			case "json": return `watcharr-export-${date}.json`;
			case "text": return `watcharr-export-${date}.txt`;
		}
	}

	function getMimeType(): string {
		switch (format) {
			case "csv": return "text/csv";
			case "json": return "application/json";
			case "text": return "text/plain";
		}
	}

	function handleDownload() {
		const content = getExportContent();
		const blob = new Blob([content], { type: getMimeType() });
		const url = URL.createObjectURL(blob);
		const a = document.createElement("a");
		a.href = url;
		a.download = getFilename();
		a.click();
		URL.revokeObjectURL(url);
	}

	function handleCopy() {
		navigator.clipboard.writeText(getExportContent());
	}

	let preview = $derived(getExportContent());
</script>

<Modal title="Export List" {onClose}>
	<div class="export-modal">
		<div class="format-picker">
			<button class:active={format === "csv"} onclick={() => format = "csv"}>CSV</button>
			<button class:active={format === "json"} onclick={() => format = "json"}>JSON</button>
			<button class:active={format === "text"} onclick={() => format = "text"}>Text</button>
		</div>

		<div class="export-info">
			{items.filter(m => m && m.name).length} items
		</div>

		<pre class="export-preview">{preview.slice(0, 2000)}{preview.length > 2000 ? "\n..." : ""}</pre>

		<div class="export-actions">
			<button class="export-btn primary" onclick={handleDownload}>
				<Icon i="download" wh={16} />
				Download
			</button>
			<button class="export-btn" onclick={handleCopy}>
				Copy to Clipboard
			</button>
		</div>
	</div>
</Modal>

<style lang="scss">
	.export-modal {
		display: flex;
		flex-direction: column;
		gap: 12px;
		width: 100%;
	}

	.format-picker {
		display: flex;
		gap: 4px;
		background: rgba(255, 255, 255, 0.04);
		border-radius: 6px;
		padding: 3px;

		button {
			flex: 1;
			padding: 6px 12px;
			border: none;
			border-radius: 4px;
			background: transparent;
			color: $text-color;
			font-size: 13px;
			cursor: pointer;
			transition: background 120ms;

			&:hover {
				background: rgba(255, 255, 255, 0.08);
			}

			&.active {
				background: rgba(255, 255, 255, 0.15);
				font-weight: 600;
			}
		}
	}

	.export-info {
		font-size: 12px;
		opacity: 0.5;
	}

	.export-preview {
		max-height: 250px;
		overflow: auto;
		background: rgba(0, 0, 0, 0.3);
		border: 1px solid rgba(255, 255, 255, 0.08);
		border-radius: 6px;
		padding: 10px;
		font-size: 11px;
		font-family: monospace;
		color: $text-color;
		white-space: pre-wrap;
		word-break: break-all;
		margin: 0;
	}

	.export-actions {
		display: flex;
		gap: 8px;
		justify-content: flex-end;
	}

	.export-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px 16px;
		border-radius: 6px;
		border: 1px solid rgba(255, 255, 255, 0.15);
		background: rgba(255, 255, 255, 0.06);
		color: $text-color;
		fill: $text-color;
		font-size: 13px;
		cursor: pointer;
		transition: background 120ms;

		:global(svg) {
			width: 16px;
			height: 16px;
		}

		&:hover {
			background: rgba(255, 255, 255, 0.12);
		}

		&.primary {
			background: rgba(96, 165, 250, 0.2);
			border-color: rgba(96, 165, 250, 0.3);

			&:hover {
				background: rgba(96, 165, 250, 0.3);
			}
		}
	}
</style>
