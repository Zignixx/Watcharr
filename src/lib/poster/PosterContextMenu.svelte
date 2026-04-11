<script lang="ts">
	import {
		type WatchedStatus,
		type Watched,
		type SupportedMedia,
		type Media,
	} from "@/types";
	import { updateWatched, removeWatched } from "../util/api";
	import Icon from "../Icon.svelte";
	import { watchedStatuses, toUnderstandableStatus } from "../util/helpers";
	import Rating from "../rating/Rating.svelte";
	import { toShowableRating } from "../rating/helpers";
	import { onMount, onDestroy } from "svelte";
	import { goto } from "$app/navigation";
	import axios from "axios";
	import SpinnerTiny from "../SpinnerTiny.svelte";
	import { store } from "@/store.svelte";

	interface Props {
		x: number;
		y: number;
		watched: Watched | undefined;
		contentId: number;
		contentType: SupportedMedia;
		mediaName: string;
		onClose: () => void;
		onWatchedUpdate: (w: Watched | undefined) => void;
		ownerWatched?: Watched;
		ownerName?: string;
	}

	let {
		x,
		y,
		watched,
		contentId,
		contentType,
		mediaName,
		onClose,
		onWatchedUpdate,
		ownerWatched,
		ownerName,
	}: Props = $props();

	let menuEl: HTMLDivElement | undefined = $state();
	let thoughtsOpen = $state(!!watched?.thoughts);
	let thoughtsText = $state(watched?.thoughts ?? "");
	let textarea: HTMLTextAreaElement | undefined = $state();
	let saving = $state(false);

	// Similar items state
	let similarItems: Media[] = $state([]);
	let similarLoading = $state(false);
	let similarExpanded = $state(false);

	// Similar item inline saving state (keyed by tmdbId)
	let similarSavingId: number | null = $state(null);

	// Position the menu within viewport bounds
	let menuX = $state(x);
	let menuY = $state(y);

	onMount(() => {
		if (menuEl) {
			const rect = menuEl.getBoundingClientRect();
			const vw = window.innerWidth;
			const vh = window.innerHeight;
			if (x + rect.width > vw) menuX = vw - rect.width - 10;
			if (y + rect.height > vh) menuY = vh - rect.height - 10;
			if (menuX < 0) menuX = 10;
			if (menuY < 0) menuY = 10;
		}
		document.addEventListener("mousedown", handleOutsideClick);
	});

	onDestroy(() => {
		document.removeEventListener("mousedown", handleOutsideClick);
	});

	function handleOutsideClick(e: MouseEvent) {
		if (menuEl && !menuEl.contains(e.target as Node)) {
			onClose();
		}
	}

	async function handleStatusClick(status: WatchedStatus) {
		if (status === watched?.status) return;
		saving = true;
		try {
			const w = await updateWatched(watched, {
				contentId,
				contentType,
				status,
			});
			onWatchedUpdate(w);
		} catch {}
		saving = false;
	}

	async function handleDelete() {
		if (!watched) return;
		saving = true;
		const removed = await removeWatched(watched.id);
		if (removed) {
			onWatchedUpdate(undefined);
		}
		saving = false;
		onClose();
	}

	async function handleRatingChange(newRating: number) {
		if (newRating === watched?.rating) return;
		saving = true;
		try {
			const w = await updateWatched(watched, {
				contentId,
				contentType,
				rating: newRating,
			});
			onWatchedUpdate(w);
		} catch {}
		saving = false;
	}

	async function handleThoughtsSave() {
		if (thoughtsText === (watched?.thoughts ?? "")) {
			thoughtsOpen = false;
			return;
		}
		saving = true;
		try {
			let w = watched;
			if (!w?.id) {
				w = await updateWatched(undefined, {
					contentId,
					contentType,
					status: "PLANNED" as WatchedStatus,
				});
				if (w) onWatchedUpdate(w);
			}
			if (w?.id) {
				const updated = await updateWatched(w, {
					contentId,
					contentType,
					thoughts: thoughtsText,
				});
				onWatchedUpdate(updated);
			}
			thoughtsOpen = false;
		} catch {}
		saving = false;
	}

	function resizeTextarea() {
		if (!textarea) return;
		textarea.style.height = "";
		textarea.style.height = textarea.scrollHeight + "px";
	}

	$effect(() => {
		if (textarea) resizeTextarea();
	});

	async function fetchSimilar() {
		if (similarLoading || similarItems.length > 0) return;
		if (contentType !== "movie" && contentType !== "tv") return;
		similarLoading = true;
		try {
			const resp = (await axios.get(`/content/${contentType}/${contentId}`, {
				params: { region: store.userSettings?.country },
			})).data as Media;
			if (resp?.similar) {
				similarItems = resp.similar.slice(0, 6);
			}
		} catch {
			// Silently fail - similar items are not critical
		}
		similarLoading = false;
	}

	function handleSimilarClick(item: Media) {
		onClose();
		const id = item.ids?.tmdb;
		if (id) {
			const type = item.type === "tmdb_movie" ? "movie" : item.type === "tmdb_tv" ? "tv" : contentType;
			goto(`/${type}/${id}`);
		}
	}

	async function handleSimilarStatusChange(item: Media, status: WatchedStatus, e?: MouseEvent) {
		if (e) e.stopPropagation();
		const id = item.ids?.tmdb;
		if (!id) return;
		const type = item.type === "tmdb_movie" ? "movie" : item.type === "tmdb_tv" ? "tv" : contentType;
		similarSavingId = id;
		try {
			const w = await updateWatched(item.watched?.id ? item.watched : undefined, {
				contentId: id,
				contentType: type as SupportedMedia,
				status,
			});
			const idx = similarItems.findIndex(s => s.ids?.tmdb === id);
			if (idx !== -1) {
				similarItems[idx] = { ...similarItems[idx], watched: w };
			}
		} catch {}
		similarSavingId = null;
	}

	async function handleSimilarRemove(item: Media, e: MouseEvent) {
		e.stopPropagation();
		if (!item.watched?.id) return;
		const id = item.ids?.tmdb;
		if (!id) return;
		similarSavingId = id;
		try {
			const removed = await removeWatched(item.watched.id);
			if (removed) {
				const idx = similarItems.findIndex(s => s.ids?.tmdb === id);
				if (idx !== -1) {
					similarItems[idx] = { ...similarItems[idx], watched: undefined };
				}
			}
		} catch {}
		similarSavingId = null;
	}
</script>

<div class="ctx-backdrop" role="presentation"></div>
<div
	class="ctx-menu"
	bind:this={menuEl}
	style="left: {menuX}px; top: {menuY}px;"
	role="menu"
>
	<div class="ctx-header">
		<button class="ctx-close" onclick={onClose}>
			<Icon i="close" wh={16} />
		</button>
		<span class="ctx-title">{mediaName}</span>
		<div class="ctx-close-spacer"></div>
	</div>

	<!-- Owner info (read-only) -->
	{#if ownerWatched}
		<div class="ctx-owner-section">
			<span class="ctx-label">{ownerName ? `${ownerName}'s Info` : "Owner's Info"}</span>
			<div class="ctx-owner-row">
				{#if ownerWatched.status}
					<span class="ctx-owner-badge">
						<Icon i={watchedStatuses[ownerWatched.status]} wh={14} />
						{toUnderstandableStatus(ownerWatched.status, contentType === "game")}
					</span>
				{/if}
				{#if ownerWatched.rating}
					<span class="ctx-owner-badge">★ {toShowableRating(ownerWatched.rating)}</span>
				{/if}
			</div>
			{#if ownerWatched.thoughts}
				<div class="ctx-owner-thoughts">"{ownerWatched.thoughts}"</div>
			{/if}
		</div>
	{/if}

	<!-- Status Section -->
	<div class="ctx-section">
		<span class="ctx-label">Status</span>
		<div class="ctx-statuses">
			{#each Object.entries(watchedStatuses) as [statusName, icon]}
				<button
					class="ctx-status-btn"
					class:active={watched?.status === statusName}
					onclick={() => handleStatusClick(statusName as WatchedStatus)}
					disabled={saving}
					title={toUnderstandableStatus(statusName as WatchedStatus, contentType === "game")}
				>
					<Icon i={icon} wh={18} />
					<span>{toUnderstandableStatus(statusName as WatchedStatus, contentType === "game")}</span>
				</button>
			{/each}
			{#if watched}
				<button
					class="ctx-status-btn delete"
					onclick={handleDelete}
					disabled={saving}
					title="Remove from list"
				>
					<Icon i="trash" wh={18} />
					<span>Remove</span>
				</button>
			{/if}
		</div>
	</div>

	<!-- Rating Section -->
	<div class="ctx-section">
		<span class="ctx-label">Rating</span>
		<div class="ctx-rating">
			<Rating rating={watched?.rating} onChange={handleRatingChange} />
		</div>
	</div>

	<!-- Thoughts Section -->
	<div class="ctx-section">
		{#if !thoughtsOpen}
			<button
				class="ctx-thoughts-btn"
				onclick={() => { thoughtsOpen = true; }}
			>
				<Icon i="pencil" wh={16} />
				<span>
					{#if watched?.thoughts}
						Edit thoughts
					{:else}
						Set thoughts on {mediaName}
					{/if}
				</span>
			</button>
		{:else}
			<div class="ctx-thoughts-editor">
				<textarea
					bind:this={textarea}
					bind:value={thoughtsText}
					oninput={resizeTextarea}
					placeholder="Your thoughts on {mediaName}"
					rows="3"
				></textarea>
				<div class="ctx-thoughts-actions">
					<button class="ctx-save-btn" onclick={handleThoughtsSave} disabled={saving}>
						Save
					</button>
					<button class="ctx-cancel-btn" onclick={() => { thoughtsOpen = false; thoughtsText = watched?.thoughts ?? ""; }}>
						Cancel
					</button>
				</div>
			</div>
		{/if}
	</div>

	<!-- Similar Items Section -->
	{#if (contentType === "movie" || contentType === "tv")}
		<div class="ctx-section ctx-similar-section">
			{#if !similarExpanded}
				<button
					class="ctx-thoughts-btn"
					onclick={() => { similarExpanded = true; fetchSimilar(); }}
				>
					<Icon i="sparkles" wh={16} />
					<span>Similar</span>
				</button>
			{:else}
				<span class="ctx-label">Similar</span>
				{#if similarLoading}
					<div class="ctx-similar-loading">
						<SpinnerTiny />
					</div>
				{:else if similarItems.length > 0}
					<div class="ctx-similar-list">
						{#each similarItems as item}
							{@const isSaving = similarSavingId === item.ids?.tmdb}
							<div class="ctx-similar-item">
								<button
									class="ctx-similar-item-main"
									onclick={() => handleSimilarClick(item)}
									title={item.name}
								>
									{#if item.extPosterPath}
										<img
											src={"https://image.tmdb.org/t/p/w92" + item.extPosterPath}
											alt={item.name}
											class="ctx-similar-poster"
										/>
									{:else}
										<div class="ctx-similar-poster ctx-similar-no-poster">?</div>
									{/if}
									<div class="ctx-similar-info">
										<span class="ctx-similar-name">{item.name}</span>
										{#if item.releaseDate}
											<span class="ctx-similar-year">{new Date(item.releaseDate).getFullYear()}</span>
										{/if}
									</div>
								</button>
								<div class="ctx-similar-statuses">
									{#each Object.entries(watchedStatuses) as [statusName, icon]}
										<button
											class="plain ctx-similar-status-btn"
											class:active={item.watched?.status === statusName}
											onclick={(e) => handleSimilarStatusChange(item, statusName as WatchedStatus, e)}
											disabled={isSaving}
											title={toUnderstandableStatus(statusName as WatchedStatus, false)}
										>
											<Icon i={icon} wh={13} />
											<span>{toUnderstandableStatus(statusName as WatchedStatus, false)}</span>
										</button>
									{/each}
									{#if item.watched?.id}
										<button
											class="plain ctx-similar-status-btn delete"
											onclick={(e) => handleSimilarRemove(item, e)}
											disabled={isSaving}
											title="Remove"
										>
											<Icon i="trash" wh={13} />
											<span>remove</span>
										</button>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{:else}
					<span class="ctx-similar-empty">No similar items found.</span>
				{/if}
			{/if}
		</div>
	{/if}
</div>

<style lang="scss">
	.ctx-backdrop {
		position: fixed;
		inset: 0;
		z-index: 99998;
		background: transparent;
	}

	.ctx-menu {
		position: fixed;
		z-index: 99999;
		background: $bg-color;
		border: 1px solid rgba(255, 255, 255, 0.12);
		border-radius: 8px;
		min-width: 260px;
		max-width: 420px;
		box-shadow: 0 8px 30px rgba(0, 0, 0, 0.5);
		overflow: hidden;
		font-size: 13px;
		color: $text-color;
	}

	.ctx-header {
		display: flex;
		align-items: center;
		padding: 10px 12px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.08);

		.ctx-title {
			font-weight: 600;
			font-size: 13px;
			white-space: nowrap;
			overflow: hidden;
			text-overflow: ellipsis;
			flex: 1;
			text-align: center;
			min-width: 0;
		}

		.ctx-close, .ctx-close-spacer {
			width: 24px;
			height: 24px;
			flex-shrink: 0;
		}

		.ctx-close {
			display: flex;
			align-items: center;
			justify-content: center;
			background: none;
			border: none;
			cursor: pointer;
			fill: $text-color;
			opacity: 0.6;
			padding: 2px;
			border-radius: 4px;
			transition: opacity 150ms;

			:global(svg) {
				width: 16px;
				height: 16px;
			}

			&:hover {
				opacity: 1;
			}
		}
	}

	.ctx-section {
		padding: 8px 12px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.06);

		&:last-child {
			border-bottom: none;
		}
	}

	.ctx-owner-section {
		padding: 8px 12px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.06);
		background: rgba(255, 255, 255, 0.03);
	}

	.ctx-owner-row {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.ctx-owner-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 3px 8px;
		border-radius: 4px;
		background: rgba(255, 255, 255, 0.08);
		font-size: 12px;
		fill: $text-color;
		text-transform: capitalize;

		:global(svg) {
			width: 14px;
			height: 14px;
			flex-shrink: 0;
		}
	}

	.ctx-owner-thoughts {
		margin-top: 6px;
		font-size: 12px;
		font-style: italic;
		opacity: 0.7;
		line-height: 1.4;
		white-space: pre-wrap;
		word-break: break-word;
	}

	.ctx-label {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		opacity: 0.5;
		margin-bottom: 6px;

		em {
			font-style: normal;
			opacity: 0.8;
			margin-left: auto;
			text-transform: none;
			letter-spacing: 0;
			font-size: 12px;
		}
	}

	.ctx-statuses {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 4px;
	}

	.ctx-status-btn {
		display: flex;
		align-items: center;
		gap: 5px;
		padding: 4px 8px;
		border-radius: 5px;
		background: rgba(255, 255, 255, 0.06);
		border: 1px solid transparent;
		cursor: pointer;
		color: $text-color;
		fill: $text-color;
		font-size: 12px;
		transition:
			background 150ms,
			border-color 150ms;

		:global(svg) {
			width: 16px;
			height: 16px;
			flex-shrink: 0;
		}

		&:hover {
			background: rgba(255, 255, 255, 0.12);
		}

		&.active {
			background: rgba(255, 255, 255, 0.15);
			border-color: rgba(255, 255, 255, 0.3);
		}

		&.delete {
			color: rgba(244, 67, 54, 0.8);
			fill: rgba(244, 67, 54, 0.8);

			&:hover {
				background: rgba(244, 67, 54, 0.15);
			}
		}

		span {
			text-transform: capitalize;
		}

		&:disabled {
			opacity: 0.5;
			pointer-events: none;
		}
	}

	.ctx-rating {
		display: flex;
		justify-content: center;
		overflow: hidden;

		:global(.wrap) {
			width: 100%;
		}
	}

	.ctx-save-btn {
		padding: 4px 10px;
		font-size: 11px;
		border-radius: 4px;
		background: rgba(255, 255, 255, 0.12);
		border: 1px solid rgba(255, 255, 255, 0.2);
		color: $text-color;
		cursor: pointer;
		transition:
			background 150ms,
			opacity 150ms;
		white-space: nowrap;

		&:hover:not(:disabled) {
			background: rgba(255, 255, 255, 0.2);
		}

		&:disabled {
			opacity: 0.4;
			cursor: default;
		}
	}

	.ctx-cancel-btn {
		padding: 4px 10px;
		font-size: 11px;
		border-radius: 4px;
		background: none;
		border: 1px solid rgba(255, 255, 255, 0.1);
		color: $text-color;
		cursor: pointer;
		opacity: 0.6;
		transition: opacity 150ms;

		&:hover {
			opacity: 1;
		}
	}

	.ctx-thoughts-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		padding: 6px 8px;
		border-radius: 5px;
		background: rgba(255, 255, 255, 0.06);
		border: none;
		cursor: pointer;
		color: $text-color;
		fill: $text-color;
		font-size: 12px;
		transition: background 150ms;

		:global(svg) {
			width: 16px;
			height: 16px;
			flex-shrink: 0;
		}

		&:hover {
			background: rgba(255, 255, 255, 0.12);
		}

		span {
			white-space: nowrap;
			overflow: hidden;
			text-overflow: ellipsis;
		}
	}

	.ctx-thoughts-editor {
		display: flex;
		flex-direction: column;
		gap: 6px;

		textarea {
			width: 100%;
			min-height: 60px;
			max-height: 150px;
			padding: 8px;
			border-radius: 5px;
			border: 1px solid rgba(255, 255, 255, 0.15);
			background: rgba(255, 255, 255, 0.06);
			color: $text-color;
			font-size: 12px;
			resize: none;
			outline: none;
			font-family: inherit;

			&:focus {
				border-color: rgba(255, 255, 255, 0.3);
			}
		}

		.ctx-thoughts-actions {
			display: flex;
			gap: 6px;
			justify-content: flex-end;
		}
	}

	.ctx-similar-section {
		max-height: 250px;
		overflow-y: auto;
	}

	.ctx-similar-loading {
		display: flex;
		justify-content: center;
		padding: 8px 0;
	}

	.ctx-similar-list {
		display: flex;
		flex-direction: column;
		gap: 3px;
	}

	.ctx-similar-item {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 6px 8px;
		border-radius: 6px;
		transition: background 150ms;

		&:hover {
			background: rgba(255, 255, 255, 0.06);
		}
	}

	.ctx-similar-item-main {
		display: flex;
		align-items: center;
		gap: 8px;
		background: none;
		border: none;
		cursor: pointer;
		color: $text-color;
		padding: 0;
		text-align: left;
		width: 100%;
	}

	.ctx-similar-poster {
		width: 32px;
		height: 48px;
		border-radius: 3px;
		object-fit: cover;
		flex-shrink: 0;
	}

	.ctx-similar-no-poster {
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(255, 255, 255, 0.08);
		font-size: 14px;
		opacity: 0.4;
	}

	.ctx-similar-info {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 1px;
	}

	.ctx-similar-name {
		font-size: 12px;
		font-weight: 500;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.ctx-similar-year {
		font-size: 11px;
		opacity: 0.5;
	}

	.ctx-similar-statuses {
		display: flex;
		flex-wrap: wrap;
		gap: 3px;
	}

	.ctx-similar-status-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 3px;
		padding: 3px 4px;
		border-radius: 4px;
		border: 1px solid rgba(255, 255, 255, 0.08);
		background: rgba(255, 255, 255, 0.03);
		cursor: pointer;
		fill: rgba(255, 255, 255, 0.4);
		color: rgba(255, 255, 255, 0.4);
		font-size: 10px;
		text-transform: capitalize;
		white-space: nowrap;
		transition: background 150ms, fill 150ms, color 150ms, border-color 150ms;

		:global(svg) {
			width: 13px;
			height: 13px;
			flex-shrink: 0;
		}

		&:hover:not(:disabled) {
			background: rgba(255, 255, 255, 0.12);
			fill: rgba(255, 255, 255, 0.8);
			color: rgba(255, 255, 255, 0.8);
			border-color: rgba(255, 255, 255, 0.15);
		}

		&.active {
			background: rgba(76, 175, 80, 0.2);
			fill: rgba(76, 175, 80, 0.9);
			color: rgba(76, 175, 80, 0.9);
			border-color: rgba(76, 175, 80, 0.4);
		}

		&.delete {
			fill: rgba(255, 255, 255, 0.3);
			color: rgba(255, 255, 255, 0.3);

			&:hover:not(:disabled) {
				fill: rgba(244, 67, 54, 0.8);
				color: rgba(244, 67, 54, 0.8);
				background: rgba(244, 67, 54, 0.1);
				border-color: rgba(244, 67, 54, 0.3);
			}
		}

		&:disabled {
			opacity: 0.4;
			cursor: not-allowed;
		}
	}

	.ctx-similar-empty {
		display: block;
		text-align: center;
		font-size: 12px;
		opacity: 0.4;
		padding: 6px 0;
	}
</style>
