# Evidence on hooks: what was measured, not opined

Two public sources with numbers. Use them to choose **which emotion** the hook pulls and to justify
an A/B test — not to promise CTR.

## 1. Upworthy Research Archive — 32.487 headline A/B tests

| Fact | Value |
|---|---|
| Experiments (package tests) | **32.487** |
| Arms/experiment packages | **150.817** |
| Period | **24/Jan/2013 – 30/Apr/2015** |
| Impressions | **538.272.878** |
| Clicks | **8.182.674** |

Source: *The Upworthy Research Archive, a time series of 32,487 experiments in U.S. media*,
Scientific Data (2021), `https://www.nature.com/articles/s41597-021-00934-7`.

What this allows you to state: there is a large corpus of headlines **randomized with measured CTR**.
What it **does not** allow: predicting your CTR. It is U.S. media headlines, from 2013-2015, for the
general reader — the attention distribution is not that of a professional feed in 2026.

Correct use: the lesson is the **loop** — two variants, same audience, the winner stays. The difference
between headlines of the same text in the archive is large precisely because the **chosen word** moves
more than the template.

## 2. Berger & Milkman — emotion and virality

Study: *What Makes Online Content Viral?* (Journal of Marketing Research, 2012; MSI Report version
10-114, `https://www.msi.org/wp-content/uploads/2020/06/MSI_Report_10-114.pdf`).

Method, so that you can weigh the evidence:

- **6.956 articles** from the New York Times, published between **30/Aug/2008 and 30/Nov/2008**, captured
  by a crawler every 15 minutes.
- Dependent variable: making the NYT **"most emailed"** list — **20%** of the articles made it.
- Specific emotions coded by **3 independent raters blind** to the hypotheses,
  5-point Likert scale, in a random subsample of **2.566 articles** (α > .70).
- Valence/emotionality measured by text analysis with a list of **7.630 words**.
- Logistic regression with day fixed effects, standard errors clustered by day.

Findings:

| Emotion / trait | Effect on the chance of going viral |
|---|---|
| Emotional content (positive **or** negative) | **+** |
| Positive valence | **+** (positive goes viral more than negative) |
| Awe | **+** |
| Surprise | **+** |
| Anger | **+** — strongest predictor: **+1 SD → 1,5× more chance** of making the list |
| Anxiety | **+** |
| Sadness | **−** |
| Disgust | no significant relationship |

## Rules that come out of it

1. **Choose the emotion before writing the sentence.** A hook is not "impactful" in the abstract: it is
   awe, surprise, anger or anxiety. Say which one you are aiming at and write for it.
2. **Sadness brings sharing down.** A technical post that opens in a tone of lament (“after months
   lost…”) asks for pity, not reach. If the story is one of failure, the hook is the **lesson**, not the pain.
3. **Anger travels, but it has a price.** For a technical audience, the defensible equivalent is the
   *contrarian claim with evidence* — anger at a practice, never at a named person.
4. **Surprise is the cheapest to produce with honesty**: a number that contradicts intuition, or the
   part that nobody tells. It is the hook that depends least on personal brand.
5. **Test in pairs.** Two variants, same audience and same time. Without a pair, you are not
   measuring a hook — you are publishing.
6. **Do not transfer a number between platforms.** “1,5×” is the relative chance of making an e-mail
   list in 2008, not CTR on LinkedIn. The relative effect guides the choice; the absolute value is fiction.

## What is not measured here yet

- Effect of **hook length** and of the cutoff point per platform: the limits are in
  `social-check/references/platform-limits.md`, but the effect of each cutoff was not measured.
- Difference in hook between LinkedIn, Reddit and X with public data: I did not find a dataset equivalent
  to the archive for those platforms. If you run your own pairs, you become the source.
