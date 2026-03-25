<script lang="ts">
	import Spinner from "@/lib/Spinner.svelte";
	import { type Media, type WatchedStatus } from "@/types";
	import axios from "axios";
	import Activity from "@/lib/Activity.svelte";
	import Title from "@/lib/content/Title.svelte";
	import Error from "@/lib/Error.svelte";
	import FollowedThoughts from "@/lib/content/FollowedThoughts.svelte";
	import { removeWatched, updateWatched, baseURL } from "@/lib/util/api.js";
	import tooltip from "@/lib/actions/tooltip.js";
	import Icon from "@/lib/Icon.svelte";
	import AddToTagButton from "@/lib/tag/AddToTagButton.svelte";
	import MyReview from "@/lib/content/MyReview.svelte";
	import PosterImage from "@/lib/content/PosterImage.svelte";
	import ExpandableText from "@/lib/content/ExpandableText.svelte";
	import WatchedDeleteBtn from "@/lib/content/WatchedDeleteBtn.svelte";

	let { data } = $props();

	let manga: Media | undefined = $state();
	let pageError: Error | undefined = $state();

	$effect(() => {
		(async () => {
			try {
				manga = undefined;
				pageError = undefined;
				if (!data.mangaId) {
					return;
				}
				const resp = (await axios.get(`/manga/${data.mangaId}`))
					.data as Media;
				manga = resp;
			} catch (err: any) {
				manga = undefined;
				pageError = err;
			}
		})();
	});

	function getPosterSrc(m: Media): string | undefined {
		if (m.poster?.path) {
			return `${baseURL}/${m.poster.path}`;
		}
		return m.extPosterPath;
	}

	async function contentChanged(
		newStatus?: WatchedStatus,
		newRating?: number,
		newThoughts?: string,
		pinned?: boolean,
	): Promise<boolean> {
		try {
			if (!data.mangaId) {
				console.error("contentChanged: no mangaId");
				return false;
			}
			if (!manga) {
				console.error("contentChanged: no manga");
				return false;
			}
			manga.watched = await updateWatched(manga.watched, {
				contentId: data.mangaId,
				contentType: "manga",
				status: newStatus,
				rating: newRating,
				thoughts: newThoughts,
				pinned: pinned,
			});
			return true;
		} catch {
			return false;
		}
	}
</script>

<svelte:head>
	<title>{manga?.name ? `${manga.name} - ` : ""}Manga</title>
</svelte:head>

{#if pageError}
	<Error pretty="Failed to load manga!" error={pageError} />
{:else if !manga}
	<Spinner />
{:else if Object.keys(manga).length > 0}
	<div>
		<div class="content">
			<div class="details-wrap">
				<div class="details-container">
					<PosterImage src={getPosterSrc(manga)} />

					<div class="details">
						<Title
							title={manga.name}
							releaseDate={manga.releaseDate
								? new Date(manga.releaseDate)
								: undefined}
							voteAverage={manga.rating}
							voteCount={manga.ratingCount}
						/>

						<span class="quick-info">
							<span class="type-badge">Manga</span>
							{#if manga.genres && manga.genres?.length > 0}
								<div>
									{#each manga.genres as g, i}
										<span
											>{g.name}{i !== manga.genres.length - 1
												? ", "
												: ""}</span
										>
									{/each}
								</div>
							{:else}
								<span>Unknown Genres</span>
							{/if}
						</span>

						<div class="manga-meta">
							{#if manga.mangaStatus}
								<span class="meta-tag">{manga.mangaStatus}</span>
							{/if}
							{#if manga.mangaChapters}
								<span class="meta-tag"
									>{manga.mangaChapters} Chapters</span
								>
							{/if}
							{#if manga.mangaVolumes}
								<span class="meta-tag">{manga.mangaVolumes} Volumes</span>
							{/if}
						</div>

						{#if manga.mangaAuthors && manga.mangaAuthors.length > 0}
							<div class="authors">
								<span class="label">Authors:</span>
								{#each manga.mangaAuthors as a, i}
									<span
										>{a}{i !== manga.mangaAuthors.length - 1
											? ", "
											: ""}</span
									>
								{/each}
							</div>
						{/if}

						<ExpandableText
							text={manga.summary}
							style="margin-bottom: 18px;"
						/>

						<div class="btns">
							{#if manga.watched}
								<div class="other-side">
									<AddToTagButton watchedItem={manga.watched} />
									<button
										onclick={() => {
											if (manga?.watched?.pinned) {
												contentChanged(
													undefined,
													undefined,
													undefined,
													false,
												);
											} else {
												contentChanged(
													undefined,
													undefined,
													undefined,
													true,
												);
											}
										}}
										use:tooltip={{
											text: `${manga.watched?.pinned ? "Unpin from" : "Pin to"} top of list`,
											pos: "bot",
										}}
									>
										<Icon
											i={manga.watched?.pinned ? "unpin" : "pin"}
											wh={19}
										/>
									</button>
									<WatchedDeleteBtn
										watchedId={manga.watched.id}
										mediaName={manga.name}
										onDelete={() => {
											if (manga) {
												manga.watched = undefined;
											}
										}}
									/>
								</div>
							{/if}
						</div>
					</div>
				</div>
			</div>

			<MyReview
				watched={manga.watched}
				contentTitle={manga.name}
				onRatingChanged={(n) => contentChanged(undefined, n)}
				onStatusChanged={(n) => contentChanged(n)}
				onThoughtsChanged={(newThoughts) => {
					return contentChanged(undefined, undefined, newThoughts);
				}}
			/>
		</div>

		<div class="page">
			{#if data.mangaId}
				<FollowedThoughts mediaType="manga" mediaId={data.mangaId} />
			{/if}

			{#if manga.watched}
				<Activity bind:activity={manga.watched.activity} />
			{/if}
		</div>
	</div>
{:else}
	<Error error="Manga not found" pretty="Manga not found" />
{/if}

<style lang="scss">
	@use "../../../../lib/content/page.scss";

	.content {
		position: relative;
		color: white;

		.details-container .details {
			.quick-info {
				display: flex;
				gap: 10px;
				margin-bottom: 8px;

				.type-badge {
					background: rgba(255, 255, 255, 0.15);
					padding: 2px 8px;
					border-radius: 4px;
					font-size: 0.85em;
					font-weight: 600;
				}
			}

			.manga-meta {
				display: flex;
				flex-flow: row;
				flex-wrap: wrap;
				gap: 6px;
				margin-bottom: 8px;

				.meta-tag {
					padding: 3px 8px;
					border-radius: 6px;
					background: rgba(255, 255, 255, 0.15);
					font-size: 13px;
				}
			}

			.authors {
				margin-bottom: 8px;
				font-size: 14px;

				.label {
					font-weight: 600;
					margin-right: 4px;
				}
			}

			.btns {
				display: flex;
				flex-flow: row;
				flex-wrap: wrap;
				gap: 8px;
				margin-top: auto;

				button {
					max-width: fit-content;
					overflow: hidden;
					animation: 50ms cubic-bezier(0.86, 0, 0.07, 1) forwards otherbtn;
					white-space: nowrap;
					gap: 6px;
				}

				.other-side {
					display: flex;
					flex-flow: row;
					gap: 8px;
					margin-left: auto;
				}
			}
		}
	}

	.page {
		max-width: 1200px;
		margin: 0 auto;
		width: 100%;
	}

	@keyframes otherbtn {
		0% {
			max-width: 0;
		}
		100% {
			max-width: 500px;
		}
	}
</style>
