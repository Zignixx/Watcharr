<script lang="ts">
	import {
		type WatchedStatus,
		type Watched,
		type Media,
		type SupportedMedia,
		type Tier,
		RatingSystem,
		RatingStep,
	} from "@/types";
	import { watchedStatuses, toUnderstandableStatus } from "@/lib/util/helpers";
	import { toShowableRating } from "@/lib/rating/helpers";
	import { store } from "@/store.svelte";
	import Icon from "@/lib/Icon.svelte";
	import { onMount } from "svelte";
	import axios from "axios";

	interface Props {
		media: Media;
		watched: Watched | undefined;
		contentType: SupportedMedia;
		poster: string | undefined;
		/** The touch point where the gesture started */
		startX: number;
		startY: number;
		onComplete: (action: {
			type: "rating";
			value: number;
		} | {
			type: "status";
			value: WatchedStatus;
		} | {
			type: "tier";
			tierId: number;
		}) => void;
		onCancel: () => void;
	}

	let {
		media,
		watched,
		contentType,
		poster,
		startX,
		startY,
		onComplete,
		onCancel,
	}: Props = $props();

	const AXIS_LOCK_THRESHOLD = 20;
	const CANCEL_THRESHOLD = 15;

	// Statuses in left-to-right order
	const statusOrder: WatchedStatus[] = [
		"PLANNED",
		"WATCHING",
		"FINISHED",
		"HOLD",
		"DROPPED",
	];

	// No status icons in HUD — just colored card with label

	let axis: "none" | "vertical" | "horizontal" = $state("none");
	let currentRating = $state(watched?.rating ?? 0);
	let currentStatus = $state<WatchedStatus | undefined>(watched?.status);
	let currentX = $state(startX);
	let currentY = $state(startY);
	let confirmed = $state(false);
	let inCancelZone = $state(false);
	let hudEl: HTMLDivElement | undefined = $state();
	let cancelZoneEl: HTMLDivElement | undefined = $state();

	// Tierlist state
	let tiers: Tier[] = $state([]);
	let tiersLoading = $state(false);
	let currentTierIndex = $state(-1);
	const currentTier = $derived(tiers.length > 0 && currentTierIndex >= 0 ? tiers[currentTierIndex] : undefined);

	const isThumbs = $derived(
		store.userSettings?.ratingSystem === RatingSystem.Thumbs,
	);
	const isTierlist = $derived(
		store.userSettings?.ratingSystem === RatingSystem.Tierlist,
	);
	const displayRating = $derived(toShowableRating(currentRating));

	// Compute internal rating range + step based on user's rating system
	const ratingConfig = $derived.by(() => {
		const rs = store.userSettings?.ratingSystem;
		const step = store.userSettings?.ratingStep;

		if (rs === RatingSystem.Thumbs) {
			return { min: 1, max: 10, step: 9 }; // 2 states: 1 or 10
		}
		if (rs === RatingSystem.OutOf100) {
			return { min: 0.1, max: 10, step: 0.1 }; // display 1-100 in 1-steps
		}
		if (rs === RatingSystem.OutOf5) {
			if (step === RatingStep.Point5) {
				return { min: 1, max: 10, step: 1 }; // display 0.5-5 in 0.5-steps
			}
			if (step === RatingStep.Point1) {
				return { min: 1, max: 10, step: 0.2 }; // display 0.5-5 in 0.1-steps
			}
			return { min: 2, max: 10, step: 2 }; // display 1-5 in 1-steps
		}
		// OutOf10 (default)
		if (step === RatingStep.Point5) {
			return { min: 1, max: 10, step: 0.5 }; // 0.5-steps
		}
		if (step === RatingStep.Point1) {
			return { min: 1, max: 10, step: 0.1 }; // 0.1-steps
		}
		return { min: 1, max: 10, step: 1 }; // 1-steps
	});

	// Rating display max (for label like "X / 100")
	const ratingMax = $derived.by(() => {
		const rs = store.userSettings?.ratingSystem;
		if (rs === RatingSystem.OutOf100) return 100;
		if (rs === RatingSystem.OutOf5) return 5;
		return 10;
	});

	function handleTouchMove(e: TouchEvent) {
		e.preventDefault();
		const touch = e.touches[0];
		if (!touch) return;
		currentX = touch.clientX;
		currentY = touch.clientY;
		const dx = currentX - startX;
		const dy = currentY - startY;

		// Lock axis after threshold
		if (axis === "none") {
			const absDx = Math.abs(dx);
			const absDy = Math.abs(dy);
			if (absDx > AXIS_LOCK_THRESHOLD || absDy > AXIS_LOCK_THRESHOLD) {
				axis = absDy >= absDx ? "vertical" : "horizontal";
			}
		}

		// Check if touch is in cancel zone
		if (cancelZoneEl) {
			const rect = cancelZoneEl.getBoundingClientRect();
			inCancelZone = currentY >= rect.top;
		}

		if (axis === "vertical") {
			const cfg = ratingConfig;
			if (isThumbs) {
				// Up = thumb up (high), Down = thumb down (low)
				currentRating = -dy > 0 ? 10 : 1;
			} else if (isTierlist && tiers.length > 0) {
				// Map vertical drag to tier index (top = first tier, bottom = last)
				const range = 200;
				const clamped = Math.max(-range, Math.min(range, -dy));
				// Up = top tier (index 0), Down = bottom tier (last)
				const t = 1 - (clamped + range) / (2 * range); // 0..1 inverted
				currentTierIndex = Math.min(tiers.length - 1, Math.max(0, Math.round(t * (tiers.length - 1))));
			} else if (!isTierlist) {
				// Map drag to internal rating with proper steps
				const range = 200;
				const clamped = Math.max(-range, Math.min(range, -dy));
				const t = (clamped + range) / (2 * range); // 0..1
				const numSteps = Math.round((cfg.max - cfg.min) / cfg.step);
				const stepIndex = Math.round(t * numSteps);
				let raw = cfg.min + stepIndex * cfg.step;
				raw = Math.min(cfg.max, Math.max(cfg.min, raw));
				currentRating = Math.round(raw * 10) / 10;
			}
		} else if (axis === "horizontal") {
			// Map horizontal drag to status index
			const stepWidth = 60;
			const baseIdx = watched?.status
				? statusOrder.indexOf(watched.status)
				: 0;
			const steps = Math.round(dx / stepWidth);
			let idx = baseIdx + steps;
			// Clamp to status bounds
			idx = Math.max(0, Math.min(statusOrder.length - 1, idx));
			currentStatus = statusOrder[idx];
		}
	}

	function handleTouchEnd() {
		if (inCancelZone) {
			onCancel();
			return;
		}

		const dx = Math.abs(currentX - startX);
		const dy = Math.abs(currentY - startY);

		if (axis === "none" || (dx < CANCEL_THRESHOLD && dy < CANCEL_THRESHOLD)) {
			onCancel();
			return;
		}

		confirmed = true;
		if (axis === "vertical" && isTierlist && currentTier) {
			onComplete({ type: "tier", tierId: currentTier.id });
		} else if (axis === "vertical" && !isTierlist) {
			if (isThumbs) {
				// Thumb up = 10, Thumb down = 1
				onComplete({ type: "rating", value: currentRating });
			} else if (currentRating > 0) {
				onComplete({ type: "rating", value: currentRating });
			} else {
				onCancel();
			}
		} else if (axis === "horizontal" && currentStatus) {
			onComplete({ type: "status", value: currentStatus });
		} else {
			onCancel();
		}
	}

	// Use document-level listeners with { passive: false } so we can
	// preventDefault() to block page scrolling. Touch events follow the
	// original target (the <li>), so inline handlers on this HUD div
	// never fire for the ongoing touch.
	onMount(() => {
		const prevOverflow = document.body.style.overflow;
		const prevTouchAction = document.body.style.touchAction;
		document.body.style.overflow = "hidden";
		document.body.style.touchAction = "none";

		document.addEventListener("touchmove", handleTouchMove, { passive: false });
		document.addEventListener("touchend", handleTouchEnd);
		document.addEventListener("touchcancel", handleCancel);

		// Fetch tiers if in tierlist mode
		if (isTierlist) {
			tiersLoading = true;
			axios.get("/tierlist").then((resp) => {
				tiers = resp.data ?? [];
			}).catch(() => {
				tiers = [];
			}).finally(() => {
				tiersLoading = false;
			});
		}

		return () => {
			document.body.style.overflow = prevOverflow;
			document.body.style.touchAction = prevTouchAction;
			document.removeEventListener("touchmove", handleTouchMove);
			document.removeEventListener("touchend", handleTouchEnd);
			document.removeEventListener("touchcancel", handleCancel);
		};
	});

	function handleCancel() {
		onCancel();
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="gesture-hud"
	class:confirmed
	bind:this={hudEl}
>
	<!-- Blurred poster background -->
	{#if poster}
		<div class="ghud-bg" style="background-image: url({poster})"></div>
	{/if}
	<div class="ghud-overlay"></div>

	<div class="ghud-content">
		<!-- Title -->
		<div class="ghud-title">{media.name}</div>

		{#if axis === "none"}
			<!-- Initial state: show instruction -->
			<div class="ghud-instruction">
				<div class="ghud-arrows">
					<span class="ghud-arrow up">
						<Icon i="chevron" facing="up" wh={28} />
					</span>
					<div class="ghud-arrow-mid">
						<span class="ghud-arrow left">
							<Icon i="chevron" facing="left" wh={28} />
						</span>
						<div class="ghud-dot"></div>
						<span class="ghud-arrow right">
							<Icon i="chevron" facing="right" wh={28} />
						</span>
					</div>
					<span class="ghud-arrow down">
						<Icon i="chevron" facing="down" wh={28} />
					</span>
				</div>
				<div class="ghud-hint-row">
					<span class="ghud-hint">↕ Rate</span>
					<span class="ghud-hint">↔ Status</span>
				</div>
			</div>
		{:else if axis === "vertical"}
			<!-- Rating mode -->
			<div class="ghud-rating-display">
				{#if isThumbs}
					<div class="ghud-thumbs" class:thumb-up={currentRating >= 5} class:thumb-down={currentRating < 5}>
						<div class="ghud-thumb-icon">
							{#if currentRating >= 5}
								<Icon i="thumb-up" wh={80} />
							{:else}
								<Icon i="thumb-down" wh={80} />
							{/if}
						</div>
						<span class="ghud-thumb-label">
							{currentRating >= 5 ? "Like" : "Dislike"}
						</span>
					</div>
				{:else if isTierlist}
					{#if tiersLoading}
						<div class="ghud-disabled-msg">Loading tiers...</div>
					{:else if tiers.length === 0}
						<div class="ghud-disabled-msg">No tiers created yet</div>
					{:else}
						<!-- Tier selector -->
						<div class="ghud-tier-display">
							{#if currentTier}
								<div
									class="ghud-tier-active"
									style="background: {currentTier.color}; color: {currentTier.textColor}"
								>
									<span class="ghud-tier-name">{currentTier.name}</span>
								</div>
							{:else}
								<div class="ghud-tier-none">Drag to select tier</div>
							{/if}
							<div class="ghud-tier-track">
								{#each tiers as tier, i}
									<div
										class="ghud-tier-dot"
										class:active={i === currentTierIndex}
										style="background: {i === currentTierIndex ? tier.color : 'rgba(255,255,255,0.2)'};
											{i === currentTierIndex ? 'box-shadow: 0 0 8px ' + tier.color : ''}"
									></div>
								{/each}
							</div>
						</div>
					{/if}
				{:else}
					<div class="ghud-rating-value">{displayRating}</div>
					<div class="ghud-rating-bar">
						<div
							class="ghud-rating-fill"
							style="width: {(currentRating / 10) * 100}%"
						></div>
					</div>
					<div class="ghud-rating-label">
						<Icon i="star" wh={20} />
						<span>{displayRating} / {ratingMax}</span>
					</div>
				{/if}
			</div>
		{:else if axis === "horizontal"}
			<!-- Status mode -->
			<div class="ghud-status-display">
				<!-- Big active status card -->
				{#if currentStatus}
					<div class="ghud-status-active status-{currentStatus.toLowerCase()}">
						<span class="ghud-status-active-label">
							{toUnderstandableStatus(currentStatus, contentType === "game")}
						</span>
					</div>
				{/if}
				<!-- Status dots / mini track -->
				<div class="ghud-status-track">
					{#each statusOrder as s}
						<div
							class="ghud-status-dot"
							class:active={s === currentStatus}
							class:dot-planned={s === "PLANNED"}
							class:dot-watching={s === "WATCHING"}
							class:dot-finished={s === "FINISHED"}
							class:dot-hold={s === "HOLD"}
							class:dot-dropped={s === "DROPPED"}
						></div>
					{/each}
				</div>
				<div class="ghud-status-hint">← swipe →</div>
			</div>
		{/if}
	</div>

	<!-- Cancel zone -->
	<div class="ghud-cancel-zone" class:active={inCancelZone} bind:this={cancelZoneEl}>
		<Icon i="x" wh={20} />
		<span>Cancel</span>
	</div>
</div>

<style lang="scss">
	.gesture-hud {
		position: fixed;
		inset: 0;
		z-index: 10000;
		touch-action: none;
		user-select: none;
		-webkit-user-select: none;
		display: flex;
		align-items: center;
		justify-content: center;
		background: black;
		animation: ghud-fadein 200ms ease forwards;
	}

	@keyframes ghud-fadein {
		from { opacity: 0; }
		to { opacity: 1; }
	}

	.gesture-hud.confirmed {
		animation: ghud-confirm 300ms ease forwards;
	}

	@keyframes ghud-confirm {
		0% { opacity: 1; transform: scale(1); }
		50% { opacity: 1; transform: scale(1.02); }
		100% { opacity: 0; transform: scale(1); }
	}

	.ghud-bg {
		position: absolute;
		inset: -20px;
		background-size: cover;
		background-position: center;
		filter: blur(30px) brightness(0.3);
	}

	.ghud-overlay {
		position: absolute;
		inset: 0;
		background: rgba(0, 0, 0, 0.65);
	}

	.ghud-content {
		position: relative;
		z-index: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 24px;
		padding: 20px;
		width: 100%;
		max-width: 360px;
	}

	.ghud-title {
		font-size: 16px;
		font-weight: 600;
		color: rgba(255, 255, 255, 0.7);
		text-align: center;
		letter-spacing: 0.5px;
		text-transform: uppercase;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		max-width: 100%;
	}

	// --- INSTRUCTION STATE ---

	.ghud-instruction {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 20px;
	}

	.ghud-arrows {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 6px;
		color: rgba(255, 255, 255, 0.4);
	}

	.ghud-arrow-mid {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.ghud-arrow {
		animation: ghud-pulse 1.5s ease-in-out infinite;
	}

	.ghud-arrow.up { animation-delay: 0s; }
	.ghud-arrow.right { animation-delay: 0.2s; }
	.ghud-arrow.down { animation-delay: 0.4s; }
	.ghud-arrow.left { animation-delay: 0.6s; }

	@keyframes ghud-pulse {
		0%, 100% { opacity: 0.3; }
		50% { opacity: 0.8; }
	}

	.ghud-dot {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: rgba(255, 255, 255, 0.5);
	}

	.ghud-hint-row {
		display: flex;
		gap: 24px;
	}

	.ghud-hint {
		font-size: 13px;
		color: rgba(255, 255, 255, 0.45);
		letter-spacing: 0.5px;
	}

	// --- RATING MODE ---

	.ghud-rating-display {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
	}

	.ghud-rating-value {
		font-size: 72px;
		font-weight: 700;
		color: white;
		line-height: 1;
		font-variant-numeric: tabular-nums;
		text-shadow: 0 2px 20px rgba(255, 255, 255, 0.3);
		transition: all 100ms ease;
	}

	.ghud-rating-bar {
		width: 220px;
		height: 6px;
		border-radius: 3px;
		background: rgba(255, 255, 255, 0.12);
		overflow: hidden;
	}

	.ghud-rating-fill {
		height: 100%;
		border-radius: 3px;
		background: linear-gradient(90deg, #f44336, #ff9800, #ffc107, #8bc34a, #4caf50);
		transition: width 100ms ease;
	}

	.ghud-rating-label {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 16px;
		color: rgba(255, 255, 255, 0.6);
		fill: rgba(255, 255, 255, 0.6);
	}

	.ghud-thumbs {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
		filter: drop-shadow(0 2px 16px currentColor);
		transition: all 150ms ease;

		&.thumb-up {
			fill: rgb(76, 175, 80);
			color: rgb(76, 175, 80);
		}
		&.thumb-down {
			fill: rgb(244, 67, 54);
			color: rgb(244, 67, 54);
		}
	}

	.ghud-thumb-icon {
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.ghud-thumb-label {
		font-size: 20px;
		font-weight: 700;
		letter-spacing: 1px;
		text-transform: uppercase;
	}

	.ghud-disabled-msg {
		font-size: 16px;
		color: rgba(255, 255, 255, 0.5);
	}

	// --- TIER MODE ---

	.ghud-tier-display {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 20px;
	}

	.ghud-tier-active {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 18px 40px;
		border-radius: 12px;
		min-width: 140px;
		animation: ghud-status-pop 200ms ease;
		box-shadow: 0 4px 24px rgba(0, 0, 0, 0.4);
	}

	.ghud-tier-name {
		font-size: 36px;
		font-weight: 800;
		letter-spacing: 2px;
		text-transform: uppercase;
		text-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
	}

	.ghud-tier-none {
		font-size: 16px;
		color: rgba(255, 255, 255, 0.4);
	}

	.ghud-tier-track {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		flex-wrap: wrap;
	}

	.ghud-tier-dot {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		transition: all 200ms ease;

		&.active {
			width: 14px;
			height: 14px;
		}
	}

	// --- STATUS MODE ---

	.ghud-status-display {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 20px;
		width: 100%;
	}

	.ghud-status-active {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		padding: 20px 32px;
		border-radius: 16px;
		background: rgba(255, 255, 255, 0.08);
		border: 2px solid rgba(255, 255, 255, 0.15);
		transition: all 150ms ease;
		animation: ghud-status-pop 200ms ease;

		&.status-planned {
			fill: rgb(100, 149, 237);
			color: rgb(100, 149, 237);
			border-color: rgba(100, 149, 237, 0.4);
			background: rgba(100, 149, 237, 0.1);
		}
		&.status-watching {
			fill: rgb(255, 193, 7);
			color: rgb(255, 193, 7);
			border-color: rgba(255, 193, 7, 0.4);
			background: rgba(255, 193, 7, 0.1);
		}
		&.status-finished {
			fill: rgb(76, 175, 80);
			color: rgb(76, 175, 80);
			border-color: rgba(76, 175, 80, 0.4);
			background: rgba(76, 175, 80, 0.1);
		}
		&.status-hold {
			fill: rgb(255, 152, 0);
			color: rgb(255, 152, 0);
			border-color: rgba(255, 152, 0, 0.4);
			background: rgba(255, 152, 0, 0.1);
		}
		&.status-dropped {
			fill: rgb(244, 67, 54);
			color: rgb(244, 67, 54);
			border-color: rgba(244, 67, 54, 0.4);
			background: rgba(244, 67, 54, 0.1);
		}
	}

	@keyframes ghud-status-pop {
		0% { transform: scale(0.9); opacity: 0.5; }
		100% { transform: scale(1); opacity: 1; }
	}

	.ghud-status-active-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		filter: drop-shadow(0 2px 12px currentColor);
	}

	.ghud-status-active-label {
		font-size: 18px;
		font-weight: 700;
		text-transform: capitalize;
		letter-spacing: 1px;
	}

	.ghud-status-track {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 12px;
	}

	.ghud-status-dot {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: rgba(255, 255, 255, 0.2);
		transition: all 200ms ease;

		&.active {
			width: 14px;
			height: 14px;
			box-shadow: 0 0 8px currentColor;
		}

		&.dot-planned { &.active { background: rgb(100, 149, 237); } }
		&.dot-watching { &.active { background: rgb(255, 193, 7); } }
		&.dot-finished { &.active { background: rgb(76, 175, 80); } }
		&.dot-hold { &.active { background: rgb(255, 152, 0); } }
		&.dot-dropped { &.active { background: rgb(244, 67, 54); } }
	}

	.ghud-status-hint {
		font-size: 12px;
		color: rgba(255, 255, 255, 0.3);
		letter-spacing: 1px;
	}

	// --- CANCEL ZONE ---

	.ghud-cancel-zone {
		position: absolute;
		bottom: 0;
		left: 0;
		right: 0;
		z-index: 2;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 20px;
		color: rgba(255, 255, 255, 0.35);
		fill: rgba(255, 255, 255, 0.35);
		font-size: 14px;
		font-weight: 600;
		letter-spacing: 0.5px;
		transition: all 150ms ease;

		&.active {
			color: rgb(244, 67, 54);
			fill: rgb(244, 67, 54);
			background: rgba(244, 67, 54, 0.12);
		}
	}
</style>
