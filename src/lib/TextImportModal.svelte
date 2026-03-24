<script lang="ts">
	import {
		ImportResponseType,
		type ImportResponse,
		type ImportedList,
		type Media,
		type WatchedStatus,
	} from "@/types";
	import { sleep } from "@/lib/util/helpers";
	import { store } from "@/store.svelte";
	import Icon from "@/lib/Icon.svelte";
	import Modal from "@/lib/Modal.svelte";
	import SpinnerTiny from "@/lib/SpinnerTiny.svelte";
	import axios from "axios";

	interface Props {
		onClose: () => void;
	}

	let { onClose }: Props = $props();

	const statusLabels: { value: WatchedStatus; label: string }[] = [
		{ value: "PLANNED", label: "Planned" },
		{ value: "WATCHING", label: "Watching" },
		{ value: "FINISHED", label: "Finished" },
		{ value: "HOLD", label: "On Hold" },
		{ value: "DROPPED", label: "Dropped" },
	];

	let importText = $state("");
	let importContentType: "movie" | "tv" | "game" = $state("movie");
	let importGlobalStatus: WatchedStatus = $state("FINISHED");
	let importEntries: (ImportedList & { _importing?: boolean; _multiResults?: Media[] })[] = $state([]);
	let importStep: "input" | "review" | "done" = $state("input");
	let importRunning = $state(false);

	function parseImportText() {
		const yearRegex = /\((\d{4})\)/;
		const lines = importText.split("\n").map((l) => l.trim()).filter((l) => l.length > 0);
		importEntries = lines.map((line) => {
			const entry: ImportedList & { _importing?: boolean; _multiResults?: Media[] } = {
				name: line,
				type: importContentType,
				status: importGlobalStatus,
			};
			const yearMatch = line.match(yearRegex);
			if (yearMatch) {
				entry.year = Number(yearMatch[1]);
				entry.name = line.replace(yearRegex, "").trim();
			}
			return entry;
		});
		importStep = "review";
	}

	function applyGlobalStatus() {
		for (const entry of importEntries) {
			if (!entry.state) entry.status = importGlobalStatus;
		}
		importEntries = [...importEntries];
	}

	async function runImport() {
		importRunning = true;
		for (const item of importEntries) {
			if (item.state === ImportResponseType.IMPORT_SUCCESS) continue;
			item._importing = true;
			item._multiResults = undefined;
			importEntries = [...importEntries];
			try {
				if (!item.name?.trim()) {
					item.state = ImportResponseType.IMPORT_NOTFOUND;
					item._importing = false;
					importEntries = [...importEntries];
					continue;
				}
				const resp = await axios.post<ImportResponse>("/import", item);
				if (resp.data.type === ImportResponseType.IMPORT_MULTI) {
					const results = resp.data.results;
					if (!results || results.length === 0) {
						item.state = ImportResponseType.IMPORT_NOTFOUND;
					} else {
						if (item.year) {
							results.sort((a, b) => {
								const ay = a.releaseDate ? new Date(Date.parse(a.releaseDate)).getFullYear() : undefined;
								const by = b.releaseDate ? new Date(Date.parse(b.releaseDate)).getFullYear() : undefined;
								if (ay === item.year) return -1;
								if (by === item.year) return 1;
								return 0;
							});
						}
						item._multiResults = results;
						item.state = ImportResponseType.IMPORT_MULTI;
					}
				} else if (resp.data.type === ImportResponseType.IMPORT_SUCCESS) {
					item.state = ImportResponseType.IMPORT_SUCCESS;
				} else {
					item.state = resp.data.type;
				}
			} catch (err) {
				console.error("Failed to import item:", item.name, err);
				item.state = ImportResponseType.IMPORT_FAILED;
			}
			item._importing = false;
			importEntries = [...importEntries];
			await sleep(1000);
		}
		importRunning = false;
		importStep = "done";
	}

	async function pickMultiResult(entry: ImportedList & { _importing?: boolean; _multiResults?: Media[] }, media: Media) {
		entry._importing = true;
		importEntries = [...importEntries];
		try {
			const req: ImportedList = {
				name: media.name,
				type: entry.type,
				status: entry.status,
			};
			if (media.ids?.tmdb) req.tmdbId = media.ids.tmdb;
			if (media.ids?.imdb) req.imdbId = media.ids.imdb;
			if (media.ids?.igdb) req.igdbId = media.ids.igdb;
			const resp = await axios.post<ImportResponse>("/import", req);
			if (resp.data.type === ImportResponseType.IMPORT_SUCCESS) {
				entry.state = ImportResponseType.IMPORT_SUCCESS;
				entry._multiResults = undefined;
			} else {
				entry.state = resp.data.type;
			}
		} catch (err) {
			console.error("Failed to import picked item:", err);
			entry.state = ImportResponseType.IMPORT_FAILED;
		}
		entry._importing = false;
		importEntries = [...importEntries];
	}
</script>

<Modal title="Import Text List" onClose={() => { if (!importRunning) onClose(); }}>
	<div class="import-modal">
		{#if importStep === "input"}
			<div class="import-input-step">
				<label>
					Paste your titles (one per line)
					<textarea
						bind:value={importText}
						placeholder={"The Dark Knight (2008)\nBreaking Bad\nInception (2010)\n..."}
						rows="10"
					></textarea>
				</label>
				<div class="import-options">
					<label>
						Type
						<select bind:value={importContentType}>
							<option value="movie">Movie</option>
							<option value="tv">TV Show</option>
							{#if store.serverFeatures?.games}
								<option value="game">Game</option>
							{/if}
						</select>
					</label>
					<label>
						Status for all
						<select bind:value={importGlobalStatus}>
							{#each statusLabels as sl}
								<option value={sl.value}>{sl.label}</option>
							{/each}
						</select>
					</label>
				</div>
				<button class="btn-save" onclick={parseImportText} disabled={!importText.trim()}>
					Next
				</button>
			</div>
		{:else if importStep === "review" || importStep === "done"}
			<div class="import-review-step">
				{#if !importRunning && importStep === "review"}
					<div class="import-global-status-row">
						<span>Set all to:</span>
						<select bind:value={importGlobalStatus} onchange={applyGlobalStatus}>
							{#each statusLabels as sl}
								<option value={sl.value}>{sl.label}</option>
							{/each}
						</select>
						<button class="btn-add btn-sm" onclick={() => { importStep = "input"; }}>
							Back
						</button>
					</div>
				{/if}
				<div class="import-entries-list">
					{#each importEntries as entry, i}
						<div class="import-entry" class:success={entry.state === ImportResponseType.IMPORT_SUCCESS} class:failed={entry.state === ImportResponseType.IMPORT_FAILED || entry.state === ImportResponseType.IMPORT_NOTFOUND} class:exists={entry.state === ImportResponseType.IMPORT_EXISTS}>
							<div class="import-entry-info">
								<span class="import-entry-name">{entry.name}{entry.year ? ` (${entry.year})` : ""}</span>
								<span class="import-entry-status-badge">
									{#if entry._importing}
										<SpinnerTiny />
									{:else if entry.state === ImportResponseType.IMPORT_SUCCESS}
										<Icon i="check" wh={14} />
									{:else if entry.state === ImportResponseType.IMPORT_FAILED || entry.state === ImportResponseType.IMPORT_NOTFOUND}
										<Icon i="close" wh={14} />
									{:else if entry.state === ImportResponseType.IMPORT_EXISTS}
										<span class="badge-exists">Exists</span>
									{:else if entry.state === ImportResponseType.IMPORT_MULTI}
										<span class="badge-multi">Pick</span>
									{/if}
								</span>
							</div>
							{#if !entry.state && !importRunning}
								<select bind:value={entry.status} class="import-entry-status-select">
									{#each statusLabels as sl}
										<option value={sl.value}>{sl.label}</option>
									{/each}
								</select>
							{/if}
							{#if entry._multiResults && entry._multiResults.length > 0}
								<div class="import-multi-results">
									<span class="multi-label">Multiple matches found — pick one:</span>
									{#each entry._multiResults.slice(0, 5) as media}
										<button class="multi-result-btn" onclick={() => pickMultiResult(entry, media)} disabled={entry._importing}>
											<span class="multi-result-name">{media.name}</span>
											{#if media.releaseDate}
												<span class="multi-result-year">({new Date(Date.parse(media.releaseDate)).getFullYear()})</span>
											{/if}
										</button>
									{/each}
								</div>
							{/if}
						</div>
					{/each}
				</div>
				{#if importStep === "review"}
					<button class="btn-save" onclick={runImport} disabled={importRunning}>
						{#if importRunning}Importing...{:else}Import ({importEntries.length}){/if}
					</button>
				{:else}
					{@const successCount = importEntries.filter((e) => e.state === ImportResponseType.IMPORT_SUCCESS).length}
					{@const failCount = importEntries.filter((e) => e.state === ImportResponseType.IMPORT_FAILED || e.state === ImportResponseType.IMPORT_NOTFOUND).length}
					{@const multiCount = importEntries.filter((e) => e.state === ImportResponseType.IMPORT_MULTI).length}
					<div class="import-summary">
						<span class="summary-success">{successCount} imported</span>
						{#if failCount > 0}<span class="summary-fail">{failCount} failed</span>{/if}
						{#if multiCount > 0}<span class="summary-multi">{multiCount} need selection</span>{/if}
					</div>
					<button class="btn-save" onclick={onClose}>
						Done
					</button>
				{/if}
			</div>
		{/if}
	</div>
</Modal>

<style lang="scss">
	.import-modal {
		min-width: 380px;
	}

	.import-input-step {
		display: flex;
		flex-direction: column;
		gap: 14px;

		label {
			display: flex;
			flex-direction: column;
			gap: 4px;
			font-size: 12px;
			font-weight: 600;
			opacity: 0.7;
		}

		textarea {
			padding: 10px 12px;
			border-radius: 8px;
			border: 1.5px solid rgba(128, 128, 128, 0.25);
			background: rgba(128, 128, 128, 0.06);
			color: $text-color;
			font-size: 13px;
			font-family: inherit;
			resize: vertical;
			min-height: 180px;
		}

		select {
			padding: 8px 12px;
			border-radius: 8px;
			border: 1.5px solid rgba(128, 128, 128, 0.25);
			background: rgba(128, 128, 128, 0.06);
			color: $text-color;
			font-size: 13px;
		}
	}

	.import-options {
		display: flex;
		gap: 12px;

		label {
			flex: 1;
		}
	}

	.import-review-step {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.import-global-status-row {
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: 13px;
		font-weight: 500;

		select {
			padding: 6px 10px;
			border-radius: 8px;
			border: 1.5px solid rgba(128, 128, 128, 0.25);
			background: rgba(128, 128, 128, 0.06);
			color: $text-color;
			font-size: 13px;
		}
	}

	.btn-sm {
		padding: 6px 12px !important;
		font-size: 12px !important;
	}

	.import-entries-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
		max-height: 350px;
		overflow-y: auto;
		padding-right: 4px;
	}

	.import-entry {
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 8px 12px;
		border-radius: 8px;
		border: 1px solid rgba(128, 128, 128, 0.15);
		background: rgba(128, 128, 128, 0.03);
		transition: all 200ms ease;

		&.success {
			border-color: rgba(40, 167, 69, 0.4);
			background: rgba(40, 167, 69, 0.06);
		}

		&.failed {
			border-color: rgba(244, 67, 54, 0.4);
			background: rgba(244, 67, 54, 0.06);
		}

		&.exists {
			border-color: rgba(255, 193, 7, 0.4);
			background: rgba(255, 193, 7, 0.06);
		}
	}

	.import-entry-info {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.import-entry-name {
		font-size: 13px;
		font-weight: 500;
	}

	.import-entry-status-badge {
		display: flex;
		align-items: center;
		flex-shrink: 0;
	}

	.badge-exists {
		font-size: 11px;
		font-weight: 600;
		color: #ffc107;
	}

	.badge-multi {
		font-size: 11px;
		font-weight: 600;
		color: $accent-color-hover;
	}

	.import-entry-status-select {
		padding: 4px 8px;
		border-radius: 6px;
		border: 1px solid rgba(128, 128, 128, 0.2);
		background: rgba(128, 128, 128, 0.06);
		color: $text-color;
		font-size: 12px;
		width: fit-content;
	}

	.import-multi-results {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 6px 0;

		.multi-label {
			font-size: 11px;
			opacity: 0.6;
			margin-bottom: 2px;
		}
	}

	.multi-result-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		border-radius: 6px;
		border: 1px solid rgba(128, 128, 128, 0.2);
		background: rgba(128, 128, 128, 0.04);
		color: $text-color;
		cursor: pointer;
		font-size: 12px;
		text-align: left;
		transition: all 150ms ease;

		&:hover {
			background: rgba(128, 128, 128, 0.12);
			border-color: $accent-color-hover;
		}

		.multi-result-name {
			font-weight: 500;
		}

		.multi-result-year {
			opacity: 0.5;
			font-size: 11px;
		}
	}

	.import-summary {
		display: flex;
		gap: 12px;
		font-size: 13px;
		font-weight: 600;

		.summary-success {
			color: $success;
		}

		.summary-fail {
			color: #f44336;
		}

		.summary-multi {
			color: $accent-color-hover;
		}
	}

	.btn-save {
		padding: 8px 22px;
		border-radius: 10px;
		border: none;
		background: $success;
		color: white;
		cursor: pointer;
		font-size: 13px;
		font-weight: 600;
		transition: all 180ms ease;

		&:hover {
			background: $success-hover;
		}

		&:disabled {
			opacity: 0.5;
			cursor: not-allowed;
		}
	}

	.btn-add {
		padding: 8px 18px;
		border-radius: 10px;
		border: 1.5px solid rgba(128, 128, 128, 0.3);
		background: rgba(128, 128, 128, 0.08);
		color: $text-color;
		cursor: pointer;
		font-size: 13px;
		font-weight: 500;
		transition: all 180ms ease;

		&:hover {
			background: rgba(128, 128, 128, 0.15);
			border-color: rgba(128, 128, 128, 0.4);
		}
	}
</style>
