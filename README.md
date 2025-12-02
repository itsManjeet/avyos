# avyos GNU/Linux

(avyos, pronounced as "__R E L A X OS__" or "__R L X OS__") is an independent effort to build a **Safe**, **Secure**, and **Beginner-friendly** distribution of GNU/Linux for users around the globe.

avyos is available in 2 variants, each with 3 channels.

## Variants

The variation in the working and management of the core components of avyos defines its different variants.

1. **Secure**: An **Immutable** variant of avyos that uses `libostree` to manage and update the core of avyos. The entire core is treated like a git repository, updating only the files changed during different releases. Please note that in immutable distributions, you cannot change the core components.

2. **Unlocked**: A new variant of avyos that operates and behaves like a traditional Linux distribution, using `pkgupd` as a package manager. PKGUPD allows users to quickly install and update core components. Please note that the unlocked variant, like any traditional distribution, is not specifically secured but provides users more control over the components.

## Update Channels

Update channels define the frequency and stability of updates:

| Channel      | Description                                                                  | Stability                        | Frequency |
| ------------ | ---------------------------------------------------------------------------- | -------------------------------- | --------- |
| Stable       | The default channel for stable releases                                      | Maximum                          | Monthly   |
| Preview      | Updates waiting for final verification before merging into stable            | Might have edge cases            | Weekly    |
| Unstable     | Updates for beta testers and the development team to check changes on **VM** | Unstable, might break the system | Daily     |

