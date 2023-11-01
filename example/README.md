## Sketch of guide

Initially empty, guide goes through adding more functions step-by-step

1. Start by adding `Build`
   - `dagger mod install <node dep>`
   - Show how Node module gives nice interface to setting up + using node/npm (nicer than writing by hand)
   - Talk about Optional + choosing defaults
   - Use w/ `dagger call build` and `dagger download --export-path ./build`
1. Add `Test`
   - `dagger call test`
1. Add `AppContainer`, which is the "prod" container w/ minimal deps that you may actually publish
   - You've written the code out, but now you want to verify the final container works as expected
   - To help, add `Debug` and call w/ `dagger shell debug`. Can now inspect container, call `nginx`, etc.
   - Now we see `nginx` working, but we can't visit the webpage from the shell.
   - To help, add `Service` and call w/ `dagger up --native service`, open in browser
1. Finally, add `PublishContainer`
   - `dagger mod install github.com/shykes/daggerverse/ttlsh`, explain this dep

## TODO

- Use github.com/quartz-technology/daggerverse/node instead of my fork once this is merged https://github.com/quartz-technology/daggerverse/pull/1
- Remove separate `Service` function once this is merged+released and invalidates it https://github.com/dagger/dagger/pull/6039
