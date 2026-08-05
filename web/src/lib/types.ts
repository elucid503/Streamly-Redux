export type ItemKind = "channel" | "vod";

export interface Item {

  kind: ItemKind;
  id: string;
  title: string;

  poster?: string;
  caption?: string;

  boxType?: number;

  season?: number;
  episode?: number;
  episodeTitle?: string;
  description?: string;

  imdbId?: string;
  tmdbId?: number;

}

export interface Quality {

  label: string;
  url: string;

  size?: string;
  height: number;

}

export interface Playback {

  kind: "hls" | "file";
  url: string;

  qualities?: Quality[];

  provider: string;
  releaseName?: string;

  sourceIndex: number;
  sourceCount: number;

}

export interface SubtitleTrack {

  id: string;
  label: string;
  url: string;

}

export interface Participant {

  userId: string;
  name: string;
  avatar?: string;

}

export interface Actor {

  userId: string;
  name: string;

  action: string;
  at: number;

}

export interface RoomState {

  item: Item | null;
  playback: Playback | null;

  playing: boolean;

  anchorMs: number;
  anchorAt: number;

  subtitle: SubtitleTrack | null;

  queue: Item[];
  queueIndex: number;

  lastActor: Actor | null;

}

export type NoticeKind = "failover" | "error" | "action";

export interface Notice {

  id: number;
  kind: NoticeKind;
  text: string;

}

export interface ServerFrame {

  type: "welcome" | "pong" | "room" | "participants" | "notice";

  state?: RoomState;
  participants?: Participant[];

  serverTime?: number;

  t0?: number;
  t1?: number;

  kind?: NoticeKind;
  text?: string;

}

export interface ClientFrame {

  type: "hello" | "ping" | "control" | "queue";

  instanceId?: string;
  accessToken?: string;

  t0?: number;

  action?: "play" | "pause" | "seek" | "next" | "prev" | "setItem" | "setSubtitle" | "setSource" | "nextSource";
  positionMs?: number;

  item?: Item | null;
  track?: SubtitleTrack | null;

  op?: "add" | "remove" | "move";

  index?: number;
  to?: number;

}

export interface Channel {

  id: string;
  name: string;

  category?: string;
  country?: string;

  logo?: string;

  backups: number;

}

export interface Title {

  id: string;
  boxType: number;
  source?: "showbox" | "tmdb";
  tmdbId?: number;

  title: string;
  year: number;

  poster?: string;
  description?: string;
  rating?: string;
  genres?: string[];

}

export interface Episode {

  episode: number;
  title?: string;
  thumbnail?: string;
  description?: string;

}

export interface Season {

  season: number;
  episodes: Episode[];

}

export interface TitleDetail extends Title {

  banner?: string;

  imdbId?: string;
  seasons: Season[];

}

export interface IntroRange {

  startMs: number;
  endMs: number;

}

export interface MatchedChannel {

  id: string;
  name: string;
  logo?: string;

}

export interface SportsMatch {

  id: string;
  title: string;
  category: string;
  league?: string;

  homeTeam?: string;
  awayTeam?: string;
  homeLogo?: string;
  awayLogo?: string;

  homeScore?: number;
  awayScore?: number;
  statusDetail?: string;
  /** Scoreboard lifecycle when known: pre / in / post. */
  status?: "pre" | "in" | "post" | string;

  startsAt: number;
  live: boolean;

  broadcast?: string;
  broadcasts?: string[];

  channel?: MatchedChannel;

}
