<script lang="ts">
	import type { Activity, ContentType, PublicUser, WatchedStatus } from "@/types";
	import HorizontalList from "../HorizontalList.svelte";
	import Modal from "../Modal.svelte";
	import axios from "axios";
	import Spinner from "../Spinner.svelte";
	import Error from "../Error.svelte";
	import Icon from "../Icon.svelte";
	import { watchedStatuses, getOrdinalSuffix, months } from "../util/helpers";

	interface FollowThoughts {
		followedUser: PublicUser;
		thoughts: string;
		status: WatchedStatus;
		rating: number;
		activity?: Activity[];
	}

	interface Props {
		mediaType: ContentType;
		// The tmdbId for movie/tv, igdbId for games.
		mediaId: number;
	}

	let { mediaType, mediaId }: Props = $props();

	let modalShownFor: FollowThoughts | undefined = $state(undefined);

	async function getFollowsThoughts() {
		return (
			await axios.get<FollowThoughts[]>(
				`/follow/thoughts/${mediaType}/${mediaId}`,
			)
		).data;
	}

	function getActivitySummary(a: Activity): string {
		switch (a?.type) {
			case "ADDED_WATCHED":
				return "Added to list";
			case "RATING_CHANGED":
				return a.data ? `Rated ${a.data}` : "Rated";
			case "STATUS_CHANGED":
			case "STATUS_CHANGED_AUTO":
				if (a.data) {
					try {
						const data = JSON.parse(a.data);
						return `Status: ${data.status || a.data}`;
					} catch {
						return `Status: ${a.data}`;
					}
				}
				return "Status changed";
			case "IMPORTED_WATCHED":
			case "IMPORTED_WATCHED_JF":
			case "IMPORTED_WATCHED_PLEX":
				return "Synced";
			case "IMPORTED_ADDED_WATCHED":
			case "IMPORTED_ADDED_WATCHED_JF":
			case "IMPORTED_ADDED_WATCHED_PLEX":
				return "Watch date imported";
			case "FINISHED":
				return "Finished";
			default:
				return a.type?.replace(/_/g, " ").toLowerCase() || "Activity";
		}
	}

	function formatDate(dateStr: string): string {
		const d = new Date(dateStr);
		return `${d.getDate()}${getOrdinalSuffix(d.getDate())} ${months[d.getMonth()]} ${d.getFullYear()}`;
	}
</script>

{#await getFollowsThoughts()}
	<Spinner />
{:then fts}
	{#if fts?.length > 0}
		<HorizontalList title="Followed Thoughts">
			{#each fts as ft}
				<button
					class={["thoughts-card plain", ft.thoughts || ft.activity?.length ? "" : "no-thoughts"].join(
						" ",
					)}
					onclick={() => (modalShownFor = ft)}
				>
					<div>
						<h4 title={ft.followedUser.username}>{ft.followedUser.username}</h4>
						{#if ft.status}
							<div class="status-icon">
								<Icon i={watchedStatuses[ft.status]} wh={22} />
							</div>
						{/if}
						{#if ft.rating}
							<span class="rating">
								<span>*</span>
								{Number.isInteger(ft.rating) ? ft.rating : ft.rating.toFixed(2)}
							</span>
						{/if}
					</div>
					{#if ft.activity && ft.activity.length > 0}
						<div class="watch-date">
							Last: {formatDate(ft.activity[0].customDate || ft.activity[0].createdAt)}
						</div>
					{/if}
					<div class="thought">
						{ft.thoughts || "No thoughts yet."}
					</div>
				</button>
			{/each}
		</HorizontalList>
	{/if}
{:catch err}
	<Error error={err} pretty="Failed to load followed thoughts!" />
{/await}

{#if modalShownFor}
	<Modal
		title={`${modalShownFor.followedUser.username}'s Thoughts`}
		onClose={() => (modalShownFor = undefined)}
	>
		{#if modalShownFor.thoughts}
			<span>{modalShownFor.thoughts}</span>
		{:else}
			<span style="opacity: 0.5;">No thoughts shared.</span>
		{/if}
		{#if modalShownFor.activity && modalShownFor.activity.length > 0}
			<div class="modal-activity">
				<h4>Activity</h4>
				<div class="activity-list">
					{#each modalShownFor.activity.slice(0, 20) as a}
						<div class="activity-item">
							<span class="activity-msg">{getActivitySummary(a)}</span>
							<span class="activity-date">{formatDate(a.customDate || a.createdAt)}</span>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</Modal>
{/if}

<style lang="scss">
	.thoughts-card {
		display: flex;
		flex-flow: column;
		padding: 15px 20px;
		background-color: $accent-color;
		fill: $text-color;
		border-radius: 10px;
		min-width: 250px;
		max-width: 250px;
		font-size: 16px;
		transition: background-color 200ms ease;
		text-align: unset;
		overflow: hidden;
		cursor: pointer;

		& > div {
			display: flex;
			flex-flow: row;
			align-items: center;
			gap: 8px;
			margin-bottom: 3px;

			h4 {
				overflow: hidden;
				text-overflow: ellipsis;
			}

			.rating {
				display: flex;
				align-items: start;
				justify-content: center;
				gap: 5px;
				font-size: 18px;
				font-weight: bolder;

				span {
					font-family: "Rampart One";
					-webkit-text-stroke: 1px $text-color;
					font-size: 32px;
					line-height: 0.55;
					margin-top: 7px;
				}
			}

			.status-icon {
				margin-left: auto;
				margin-right: 2px;
				width: 20px;
				height: 20px;
				min-width: 20px;
				min-height: 20px;
			}
		}

		.thought {
			display: -webkit-box;
			-webkit-line-clamp: 9;
			-webkit-box-orient: vertical;
			hyphens: auto;
			overflow: hidden;
		}

		&.no-thoughts {
			pointer-events: none;

			.thought {
				font-weight: lighter;
			}
		}

		.watch-date {
			font-size: 12px;
			opacity: 0.6;
			margin-bottom: 4px;
		}

		&:hover {
			color: $bg-color;
			fill: $bg-color;
			background-color: $accent-color-hover;

			.rating span {
				-webkit-text-stroke: 1px $bg-color;
			}
		}
	}

	:global(.modal-activity) {
		margin-top: 16px;

		h4 {
			margin-bottom: 8px;
			opacity: 0.7;
			font-size: 13px;
			text-transform: uppercase;
			letter-spacing: 0.5px;
		}
	}

	:global(.activity-list) {
		display: flex;
		flex-direction: column;
		gap: 4px;
		max-height: 300px;
		overflow-y: auto;
	}

	:global(.activity-item) {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 6px 10px;
		background: rgba(255, 255, 255, 0.05);
		border-radius: 6px;
		font-size: 13px;
	}

	:global(.activity-msg) {
		text-transform: capitalize;
	}

	:global(.activity-date) {
		opacity: 0.5;
		font-size: 12px;
		white-space: nowrap;
		margin-left: 12px;
	}
</style>
